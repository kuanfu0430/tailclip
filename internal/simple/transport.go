package simple

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/tailscale/tailcat"
)

const Port = uint16(17733)

type transport struct {
	server   *tailcat.Server
	http     *http.Server
	listener *channelListener
}

func startTransport(ctx context.Context, identity *tailcat.PrivateKey, handler http.Handler) (*transport, error) {
	if len(identity.Public.Region) == 0 {
		if identity.Public.RegionID == 0 {
			identity.Public.RegionID = -1
		}
		if err := identity.Public.Expand(ctx, tailcat.ExpandForServer); err != nil {
			return nil, err
		}
	}
	listener := &channelListener{connections: make(chan net.Conn), done: make(chan struct{})}
	server := &tailcat.Server{Key: identity.Private, PresharedKey: identity.Public.PresharedKey, Region: identity.Public.Region[0], Logf: func(string, ...any) {}}
	server.OnTCP = func(port uint16) func(net.Conn) {
		if port != Port {
			return nil
		}
		return func(conn net.Conn) {
			wrapped := &trackedConn{Conn: conn, done: make(chan struct{})}
			select {
			case listener.connections <- wrapped:
				<-wrapped.done
			case <-listener.done:
				wrapped.Close()
			}
		}
	}
	if err := server.Start(); err != nil {
		listener.Close()
		return nil, err
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 12 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
	runtime := &transport{server, httpServer, listener}
	go func() { _ = httpServer.Serve(listener) }()
	return runtime, nil
}

func (t *transport) Close() { _ = t.http.Close(); _ = t.listener.Close(); _ = t.server.Close() }

type channelListener struct {
	connections chan net.Conn
	done        chan struct{}
	once        sync.Once
}

func (l *channelListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.connections:
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}
func (l *channelListener) Close() error   { l.once.Do(func() { close(l.done) }); return nil }
func (l *channelListener) Addr() net.Addr { return &net.TCPAddr{Port: int(Port)} }

type trackedConn struct {
	net.Conn
	done chan struct{}
	once sync.Once
}

func (c *trackedConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { close(c.done) })
	return err
}
