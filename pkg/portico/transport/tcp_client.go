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

	"github.com/icemagno/valhalla/pkg/portico/wire"
)

// Códigos de sincronização do transporte TCP do Portico
const (
	MagicWelcome int32 = 0xbeef // 48879
	MagicReady   int32 = 0xfeed // 65261
)

var (
	ErrInvalidWelcomeMagic = errors.New("portico transport: magic de boas-vindas invalido (esperado 0xBEEF)")
	ErrInvalidReadyCode    = errors.New("portico transport: codigo de prontidao invalido (esperado 0xFEED)")
	ErrNotConnected        = errors.New("portico transport: cliente nao conectado")
)

// TCPClient gerencia a conexao física de controle e dados TCP com o executivo Portico
type TCPClient struct {
	endpoint   string
	conn       net.Conn
	reader     *bufio.Reader
	writer     *bufio.Writer
	welcomeMsg string
	mu         sync.Mutex
	connected  bool
}

// NewTCPClient instancia um novo cliente para o endpoint informado (ex: "192.168.1.226:52295")
func NewTCPClient(endpoint string) *TCPClient {
	return &TCPClient{
		endpoint: endpoint,
	}
}

// Connect estabelece o socket TCP e realiza imediatamente o handshake 0xBEEF / 0xFEED
func (c *TCPClient) Connect(timeout time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

	// Executa a negociacao de handshake exigida pelo Portico
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

// performHandshake implementa a logica binaria reversa do TcpClientProxy.java do Portico
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
		return fmt.Errorf("tamanho de mensagem de boas-vindas absurdo: %d bytes", strLen)
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
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.welcomeMsg
}

// IsConnected indica se a conexao está ativa
func (c *TCPClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

// Close encerra a conexao graciosamente
func (c *TCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

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

// SendPacket envia um pacote completo (Cabecalho Portico + Payload)
func (c *TCPClient) SendPacket(hdr wire.Header, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return ErrNotConnected
	}

	hdr.PayloadLength = uint32(len(payload))
	hdrBuf := make([]byte, wire.HeaderSize)
	if err := hdr.MarshalTo(hdrBuf); err != nil {
		return err
	}

	if _, err := c.writer.Write(hdrBuf); err != nil {
		return err
	}

	if len(payload) > 0 {
		if _, err := c.writer.Write(payload); err != nil {
			return err
		}
	}

	return c.writer.Flush()
}

// ReadPacket aguarda e recebe o proximo pacote de rede
func (c *TCPClient) ReadPacket() (wire.Header, []byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var hdr wire.Header
	if !c.connected {
		return hdr, nil, ErrNotConnected
	}

	hdrBuf := make([]byte, wire.HeaderSize)
	if _, err := io.ReadFull(c.reader, hdrBuf); err != nil {
		return hdr, nil, err
	}

	if err := hdr.UnmarshalBinary(hdrBuf); err != nil {
		return hdr, nil, err
	}

	var payload []byte
	if hdr.PayloadLength > 0 {
		payload = make([]byte, hdr.PayloadLength)
		if _, err := io.ReadFull(c.reader, payload); err != nil {
			return hdr, nil, err
		}
	}

	return hdr, payload, nil
}
