package quictransport

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/quic-go/quic-go"
)

// Listener wraps *quic.Listener to satisfy net.Listener.
type Listener struct {
	ln *quic.Listener
}

func NewListener(addr string, tlsConf *tls.Config) (*Listener, error) {
	ln, err := quic.ListenAddr(addr, tlsConf, nil)
	if err != nil {
		return nil, err
	}
	return &Listener{ln: ln}, nil
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.ln.Accept(context.Background())
	if err != nil {
		return nil, err
	}
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}
	return &streamConn{conn: conn, Stream: stream}, nil
}

func (l *Listener) Close() error {
	return l.ln.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.ln.Addr()
}

type streamConn struct {
	conn *quic.Conn
	*quic.Stream
}

func (c *streamConn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *streamConn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }
