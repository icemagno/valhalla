package crc

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"time"
)

// MagicNativeVAL4 define os 4 bytes do cabeçalho que identifica um cliente nativo Valhalla
var MagicNativeVAL4 = []byte{'V', 'A', 'L', '4'}

// MultiplexedConn encapsula a conexão TCP e o leitor com buffer pré-alimentado pelo Peek
type MultiplexedConn struct {
	net.Conn
	Reader io.Reader
}

func (m *MultiplexedConn) Read(b []byte) (int, error) {
	return m.Reader.Read(b)
}

// DetectProtocol analisa os primeiros bytes recebidos na conexão para inferir o dialeto
func DetectProtocol(conn net.Conn, peekTimeout time.Duration) (ProtocolType, *MultiplexedConn, error) {
	br := bufio.NewReader(conn)

	// Estabelecer prazo curto para verificar se o cliente inicia ativamente
	_ = conn.SetReadDeadline(time.Now().Add(peekTimeout))
	peekBuf, err := br.Peek(4)
	_ = conn.SetReadDeadline(time.Time{}) // Resetar deadline

	wrappedConn := &MultiplexedConn{
		Conn:   conn,
		Reader: br,
	}

	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			// Timeout: O cliente não enviou bytes imediatamente.
			// Essa é a assinatura "Server-First" do Portico (Thor aguarda o 0xBEEF do servidor)
			return ProtocolPortico, wrappedConn, nil
		}
		if errors.Is(err, io.EOF) {
			return 0, wrappedConn, err
		}
		// Se foram lidos menos de 4 bytes antes do timeout
		return ProtocolPortico, wrappedConn, nil
	}

	// Cliente ativo enviou dados imediatamente:
	if bytes.Equal(peekBuf, MagicNativeVAL4) {
		// Consumir os 4 bytes "VAL4" do buffer para o pipeline nativo
		_, _ = br.Discard(4)
		return ProtocolNative, wrappedConn, nil
	}

	// Por padrão, se não é VAL4, roteia para o pipeline de compatibilidade Portico
	return ProtocolPortico, wrappedConn, nil
}
