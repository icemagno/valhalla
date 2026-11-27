package transport

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/icemagno/valhalla/pkg/crc/portico/wire"
)

// Códigos de sincronização e enquadramento do transporte TCP do Portico
const (
	MagicWelcome   int32 = 0xbeef // 48879 - Boas-vindas enviada pelo servidor Portico
	MagicReady     int32 = 0xfeed // 65261 - Sinal de prontidao bidirecional
	MagicTCPBundle int32 = 0xcafe // 52094 - Marcador de enquadramento de pacote/bundle
)

var (
	ErrInvalidWelcomeMagic = errors.New("portico transport: magic de boas-vindas invalido (esperado 0xBEEF)")
	ErrInvalidReadyCode    = errors.New("portico transport: codigo de prontidao invalido (esperado 0xFEED)")
	ErrInvalidBundleMagic  = errors.New("portico transport: marcador de bundle invalido (esperado 0xCAFE)")
	ErrNotConnected        = errors.New("portico transport: cliente nao conectado")
	ErrBundleTooLarge      = errors.New("portico transport: bundle excede tamanho maximo suportado (16 MB)")
)

// TCPClient gerencia a conexao física de controle e dados TCP com o executivo Portico (LRC/RTI).
// Suporta operacoes full-duplex com travas independentes de envio e recepcao conforme as
// melhores praticas de engenharia de software de rede em Go.
type TCPClient struct {
	endpoint   string
	conn       net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	welcomeMsg string

	stateMu   sync.RWMutex
	connected bool

	sendMu sync.Mutex
	recvMu sync.Mutex
}

// NewTCPClient instancia um novo cliente para o endpoint informado (ex: "192.168.1.226:52295")
func NewTCPClient(endpoint string) *TCPClient {
	return &TCPClient{
		endpoint: endpoint,
	}
}

// Connect estabelece o socket TCP e realiza imediatamente o handshake 0xBEEF / 0xFEED
func (c *TCPClient) Connect(timeout time.Duration) error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if c.connected {
		return nil
	}

	conn, err := net.DialTimeout("tcp", c.endpoint, timeout)
	if err != nil {
		return fmt.Errorf("falha ao conectar no endpoint Portico %s: %w", c.endpoint, err)
	}

	c.conn = conn
	c.reader = bufio.NewReader(conn)
	c.writer = bufio.NewWriter(conn)

	// Executa a negociacao de handshake exigida pelo TcpClientProxy.java do Portico
	if err := c.performHandshake(); err != nil {
		_ = c.conn.Close()
		c.conn = nil
		c.reader = nil
		c.writer = nil
		return err
	}

	c.connected = true
	return nil
}

// performHandshake implementa a logica binaria estrita do TcpClientProxy.java do Portico
func (c *TCPClient) performHandshake() error {
	// 1. Ler o magic de boas-vindas (int32 = 0xBEEF)
	var magic int32
	if err := binary.Read(c.reader, binary.BigEndian, &magic); err != nil {
		return fmt.Errorf("erro lendo magic de boas-vindas: %w", err)
	}
	if magic != MagicWelcome {
		return fmt.Errorf("%w: recebido 0x%X", ErrInvalidWelcomeMagic, magic)
	}

	// 2. Ler o tamanho da string de boas-vindas (int32)
	var strLen int32
	if err := binary.Read(c.reader, binary.BigEndian, &strLen); err != nil {
		return fmt.Errorf("erro lendo tamanho da mensagem de boas-vindas: %w", err)
	}
	if strLen < 0 || strLen > 65536 {
		return fmt.Errorf("tamanho de mensagem de boas-vindas invalido: %d bytes", strLen)
	}

	// 3. Ler o texto da mensagem de boas-vindas
	msgBuf := make([]byte, strLen)
	if _, err := io.ReadFull(c.reader, msgBuf); err != nil {
		return fmt.Errorf("erro lendo texto da mensagem de boas-vindas: %w", err)
	}
	c.welcomeMsg = string(msgBuf)

	// 4. Ler o codigo de prontidao do servidor (int32 = 0xFEED)
	var srvReady int32
	if err := binary.Read(c.reader, binary.BigEndian, &srvReady); err != nil {
		return fmt.Errorf("erro lendo codigo de prontidao do servidor: %w", err)
	}
	if srvReady != MagicReady {
		return fmt.Errorf("%w: recebido 0x%X", ErrInvalidReadyCode, srvReady)
	}

	// 5. Responder com a confirmacao do cliente (int32 = 0xFEED)
	if err := binary.Write(c.writer, binary.BigEndian, MagicReady); err != nil {
		return fmt.Errorf("erro enviando confirmacao 0xFEED ao servidor: %w", err)
	}

	if err := c.writer.Flush(); err != nil {
		return fmt.Errorf("erro descarregando buffer de handshake: %w", err)
	}

	return nil
}

// WelcomeMessage retorna a mensagem de apresentacao enviada pelo servidor Portico
func (c *TCPClient) WelcomeMessage() string {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.welcomeMsg
}

// IsConnected indica se a conexao está ativa
func (c *TCPClient) IsConnected() bool {
	c.stateMu.RLock()
	defer c.stateMu.RUnlock()
	return c.connected
}

// Close encerra a conexao graciosamente
func (c *TCPClient) Close() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()

	if !c.connected {
		return nil
	}

	c.connected = false
	var err error
	if c.conn != nil {
		err = c.conn.Close()
	}
	c.conn = nil
	c.reader = nil
	c.writer = nil
	return err
}

// SendPacket empacota um cabecalho Portico e payload no enquadramento TCP bundle oficial (0xCAFE):
//   [int32: 0xCAFE] [int32: bundleLength] [Header: 12 bytes] [Payload: N bytes]
func (c *TCPClient) SendPacket(hdr wire.Header, payload []byte) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.stateMu.RLock()
	connected := c.connected
	writer := c.writer
	c.stateMu.RUnlock()

	if !connected || writer == nil {
		return ErrNotConnected
	}

	hdr.PayloadLength = uint32(len(payload))
	bundleLength := int32(wire.HeaderLength + len(payload))

	// 1. Escrever Magic de Bundle (0xCAFE)
	if err := binary.Write(writer, binary.BigEndian, MagicTCPBundle); err != nil {
		return fmt.Errorf("falha ao escrever magic 0xCAFE: %w", err)
	}

	// 2. Escrever tamanho total do bundle (Header + Payload)
	if err := binary.Write(writer, binary.BigEndian, bundleLength); err != nil {
		return fmt.Errorf("falha ao escrever tamanho do bundle: %w", err)
	}

	// 3. Escrever o cabecalho de 12 bytes
	var hdrBuf [wire.HeaderLength]byte
	if err := hdr.MarshalTo(hdrBuf[:]); err != nil {
		return fmt.Errorf("falha ao serializar cabecalho Portico: %w", err)
	}
	if _, err := writer.Write(hdrBuf[:]); err != nil {
		return fmt.Errorf("falha ao enviar cabecalho: %w", err)
	}

	// 4. Escrever o payload (se houver)
	if len(payload) > 0 {
		if _, err := writer.Write(payload); err != nil {
			return fmt.Errorf("falha ao enviar payload: %w", err)
		}
	}

	return writer.Flush()
}

// ReadPacket aguarda e recebe o proximo pacote enquadrado com 0xCAFE:
//   [int32: 0xCAFE] [int32: bundleLength] [Header: 12 bytes] [Payload: N bytes]
func (c *TCPClient) ReadPacket() (wire.Header, []byte, error) {
	c.recvMu.Lock()
	defer c.recvMu.Unlock()

	var hdr wire.Header
	c.stateMu.RLock()
	connected := c.connected
	reader := c.reader
	c.stateMu.RUnlock()

	if !connected || reader == nil {
		return hdr, nil, ErrNotConnected
	}

	for {
		// 1. Ler o magic do bundle (int32 = 0xCAFE)
		var magic int32
		if err := binary.Read(reader, binary.BigEndian, &magic); err != nil {
			return hdr, nil, fmt.Errorf("falha lendo magic do bundle: %w", err)
		}
		if magic != MagicTCPBundle {
			return hdr, nil, fmt.Errorf("%w: recebido 0x%X", ErrInvalidBundleMagic, magic)
		}

		// 2. Ler o tamanho do bundle
		var bundleLen int32
		if err := binary.Read(reader, binary.BigEndian, &bundleLen); err != nil {
			return hdr, nil, fmt.Errorf("falha lendo tamanho do bundle: %w", err)
		}
		if bundleLen == 0 {
			// Heartbeat / Keepalive / Bundle vazio enviado pelo Portico - ignora e le o proximo
			continue
		}
		if bundleLen < wire.HeaderLength {
			return hdr, nil, fmt.Errorf("bundle menor que tamanho minimo do cabecalho (12 bytes): %d", bundleLen)
		}
		if bundleLen > 0x01000000 { // 16 MB max
			return hdr, nil, ErrBundleTooLarge
		}

		// 3. Ler o cabecalho de 12 bytes
		var hdrBuf [wire.HeaderLength]byte
		if _, err := io.ReadFull(reader, hdrBuf[:]); err != nil {
			return hdr, nil, fmt.Errorf("falha lendo cabecalho do bundle: %w", err)
		}
		if err := hdr.UnmarshalBinary(hdrBuf[:]); err != nil {
			return hdr, nil, fmt.Errorf("falha decodificando cabecalho: %w", err)
		}

		// 4. Ler o payload restante do pacote
		payloadLen := int(bundleLen) - wire.HeaderLength
		var payload []byte
		if payloadLen > 0 {
			payload = make([]byte, payloadLen)
			if _, err := io.ReadFull(reader, payload); err != nil {
				return hdr, nil, fmt.Errorf("falha lendo payload do bundle: %w", err)
			}
		}

		return hdr, payload, nil
	}
}

// ReadPacketTimeout aguarda o proximo pacote de rede com timeout configuravel
func (c *TCPClient) ReadPacketTimeout(timeout time.Duration) (wire.Header, []byte, error) {
	c.stateMu.RLock()
	conn := c.conn
	c.stateMu.RUnlock()

	if timeout > 0 && conn != nil {
		_ = conn.SetReadDeadline(time.Now().Add(timeout))
		defer func() {
			_ = conn.SetReadDeadline(time.Time{})
		}()
	}
	return c.ReadPacket()
}

