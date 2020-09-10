package gonet

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// Conn alias std library net.Conn
type Conn = net.Conn

var maxReconnectTimeout = time.Second * 60

// ErrClosedManually to exit for loop
var ErrClosedManually = errors.New("connection closed manually")

type reconnectWaitGroup struct {
	sync.WaitGroup
	err error
}

// AutoReconnectConn net.Conn implementation with auto-reconnect feature
type AutoReconnectConn struct {
	net.Conn

	ctx  context.Context
	quit context.CancelFunc
	mu   sync.Mutex
	wg   *reconnectWaitGroup

	OnConnected func(net.Conn) error
}

func newConn(conn net.Conn, connOpts ...ConnOption) *AutoReconnectConn {
	c := &AutoReconnectConn{Conn: conn}
	for _, opt := range connOpts {
		opt(c)
	}
	c.ctx, c.quit = context.WithCancel(context.Background())
	return c
}

// Close implement net.Conn interface
func (c *AutoReconnectConn) Close() error {
	c.quit()
	return c.Conn.Close()
}

func (c *AutoReconnectConn) Write(b []byte) (int, error) {
	log.Printf("Prefer to write %s", b)
	n, err := c.Conn.Write(b)
	isReconnected, err := c.reconnectIfNeeded(err)
	if err != nil {
		return 0, err
	}
	if isReconnected {
		return c.Write(b)
	}
	return n, err
}

// Read implement net.Conn interface
// Reconnect if read io.EOF or read non temporary net.Error
func (c *AutoReconnectConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	isReconnected, err := c.reconnectIfNeeded(err)
	if err != nil {
		return 0, err
	}
	if isReconnected {
		return c.Read(b)
	}
	return n, err
}

func (c *AutoReconnectConn) reconnectIfNeeded(err error) (bool, error) {
	if err == nil {
		return false, nil
	}

	var netErr net.Error
	if err == io.EOF || (errors.As(err, &netErr) && !netErr.Temporary()) {
		err = c.reconnect()
	} else {
		return false, err
	}

	return true, err
}

// The returns error is either nil or io.EOF
func (c *AutoReconnectConn) reconnect() error {

	c.mu.Lock()
	if c.wg != nil {
		wg := c.wg
		c.mu.Unlock()
		wg.Wait()
		return wg.err
	}

	c.wg = new(reconnectWaitGroup)
	c.wg.Add(1)
	c.mu.Unlock()

	// do reconnect
	raddr := c.Conn.RemoteAddr()
	timeout := time.Second

reconnect:
	for {
		select {
		case <-c.ctx.Done():
			c.wg.err = ErrClosedManually
			break reconnect
		default:
		}

		log.Printf("Reconnect timeout: %v", timeout)
		conn, err := (&net.Dialer{}).DialContext(c.ctx, raddr.Network(), raddr.String())
		if err != nil {
			timeout *= 2
			if timeout > maxReconnectTimeout {
				timeout = maxReconnectTimeout
			}
			select {
			case <-time.After(timeout):
				continue
			case <-c.ctx.Done():
				c.wg.err = ErrClosedManually
				break reconnect
			}
		}
		c.Conn = conn

		// on connected custom event callback
		if c.OnConnected != nil {
			c.wg.err = c.OnConnected(c.Conn)
		}

		break reconnect
	}

	c.mu.Lock()
	wg := c.wg
	c.wg = nil
	c.mu.Unlock()

	// release
	wg.Done()

	return wg.err
}

// DialAutoReconnectContext wrap net.DialContext
func DialAutoReconnectContext(ctx context.Context, network string, address string, connOpts ...ConnOption) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	autoConn := newConn(conn, connOpts...)

	if autoConn.OnConnected != nil {
		if err := autoConn.OnConnected(conn); err != nil {
			return nil, err
		}
	}
	return autoConn, err
}

// ConnOption dialing network optioins
type ConnOption func(*AutoReconnectConn)

// WithOnConnected when net.Conn connected
func WithOnConnected(fn func(net.Conn) error) ConnOption {
	return func(conn *AutoReconnectConn) {
		conn.OnConnected = fn
	}
}
