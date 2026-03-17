package grpc

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/quic-go/quic-go"
)

// QUICListener wraps *quic.Listener to satisfy net.Listener
type QUICListener struct {
	ln *quic.Listener
}

func NewQUICListener(addr string, tlsConf *tls.Config) (*QUICListener, error) {
	ln, err := quic.ListenAddr(addr, tlsConf, nil)
	if err != nil {
		return nil, err
	}
	return &QUICListener{ln: ln}, nil
}

func (l *QUICListener) Accept() (net.Conn, error) {
	conn, err := l.ln.Accept(context.Background())
	if err != nil {
		return nil, err
	}
	stream, err := conn.AcceptStream(context.Background())
	if err != nil {
		return nil, err
	}
	return &quicStreamConn{conn: conn, Stream: stream}, nil
}

func (l *QUICListener) Close() error {
	return l.ln.Close()
}

func (l *QUICListener) Addr() net.Addr {
	return l.ln.Addr()
}

type quicStreamConn struct {
	conn *quic.Conn
	*quic.Stream
}

func (c *quicStreamConn) LocalAddr() net.Addr  { return c.conn.LocalAddr() }
func (c *quicStreamConn) RemoteAddr() net.Addr { return c.conn.RemoteAddr() }
