package main

import (
	"bytes"
	"crypto/tls"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type client struct {
	serverHost      string
	cfg             config
	password        []byte
	keepAlivePeriod time.Duration
	dialTimeout     time.Duration
	wg              sync.WaitGroup
	errChan         chan error
	signal          chan os.Signal
	done            chan struct{}
}

func newClient(cfg config, sigChan chan os.Signal) *client {
	return &client{
		cfg:             cfg,
		keepAlivePeriod: time.Duration(cfg.KeepAlivePeriod) * time.Second,
		dialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		errChan:         make(chan error, 1),
		signal:          sigChan,
		done:            make(chan struct{}),
	}
}

func (c *client) connCopy(dst, src net.Conn, copyDone chan struct{}) {
	defer c.wg.Done()
	defer func() {
		copyDone <- struct{}{}
	}()
	_, err := io.Copy(dst, src)
	if err != nil {
		opErr, ok := err.(*net.OpError)
		switch {
		case ok && opErr.Op == "readfrom":
			return
		case ok && opErr.Op == "read":
			return
		default:
		}
		log.Println("[ERR] gsocks5: Failed to copy connection from",
			src.RemoteAddr(), "to", dst.RemoteAddr(), ":", err)
	}
}

func (c *client) proxyClientConn(conn, rConn net.Conn, ch chan struct{}) {
	defer c.wg.Done()

	// close ch, clientConn waits until it will be closed.
	defer close(ch)
	copyDone := make(chan struct{}, 2)

	c.wg.Add(2)
	go c.connCopy(rConn, conn, copyDone)
	go c.connCopy(conn, rConn, copyDone)
	// rConn and conn will be closed by defer calls in clientConn. There is nothing to do here.
	<-copyDone
}

func (c *client) authenticate(conn net.Conn, errChan chan error) {
	defer c.wg.Done()
	_, err := conn.Write(c.password)
	if err != nil {
		errChan <- nil
		return
	}
	// Wait for authSuccess
	buf := make([]byte, len(authSuccess))
	_, err = conn.Read(buf)
	if err != nil {
		errChan <- err
		return
	}
	if !bytes.Equal(buf, authSuccess) {
		errChan <- errAuthenticationFailed
		return
	}
	errChan <- nil
	return
}

func (c *client) clientConn(conn net.Conn) {
	defer c.wg.Done()
	defer closeConn(conn)

	d := &net.Dialer{
		Timeout: c.dialTimeout,
	}
	cfg := &tls.Config{
		InsecureSkipVerify: c.cfg.InsecureSkipVerify,
	}
	rConn, err := tls.DialWithDialer(d, "tcp", c.serverHost, cfg)
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to dial", c.serverHost, err)
		return
	}
	defer closeConn(rConn)

	if c.password != nil {
		errChan := make(chan error, 1)
		c.wg.Add(1)
		go c.authenticate(rConn, errChan)
		select {
		case <-time.After(5 * time.Second):
			log.Println("[ERR] gsocks5: Authentication timeout")
			return
		case err := <-errChan:
			if err != nil {
				log.Println("[ERR] gsocks5: Failed to authenticate:", err)
				return
			}
		}
	}

	ch := make(chan struct{})
	c.wg.Add(1)
	go c.proxyClientConn(conn, rConn, ch)
	select {
	case <-c.done:
	case <-ch:
	}
}

func (c *client) serve(l net.Listener) {
	defer c.wg.Done()
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("[DEBUG] gsocks5: Listener error:", err)
			// Shutdown the client immediately.
			c.shutdown()
			if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
				c.errChan <- err
				return
			}
			c.errChan <- nil
			return
		}
		conn.(*net.TCPConn).SetKeepAlive(true)
		conn.(*net.TCPConn).SetKeepAlivePeriod(c.keepAlivePeriod)
		c.wg.Add(1)
		go c.clientConn(conn)
	}
}

func (c *client) shutdown() {
	select {
	case <-c.done:
		return
	default:
	}
	close(c.done)
}

func (c *client) run() error {
	var err error
	host, port := c.cfg.ClientHost, c.cfg.ClientPort
	addr := net.JoinHostPort(host, port)
	c.serverHost = net.JoinHostPort(c.cfg.ServerHost, c.cfg.ServerPort)
	if c.cfg.Password != "" {
		c.password = []byte(c.cfg.Password)
		if len(c.password) > maxPasswordLength {
			return errPasswordTooLong
		}
	}

	rawListener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	log.Println("[INF] gsocks5: Proxy client runs on", addr)
	c.wg.Add(1)
	go c.serve(rawListener)

	select {
	// Wait for SIGINT or SIGTERM
	case <-c.signal:
	// Wait for a listener error
	case <-c.done:
	}

	// Signal all running goroutines to stop.
	c.shutdown()

	log.Println("[INF] gsocks5: Stopping proxy client", addr)
	if err = rawListener.Close(); err != nil {
		log.Println("[ERR] gsocks5: Failed to close listener", err)
	}

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		c.wg.Wait()
	}()

	select {
	case <-ch:
	case <-time.After(time.Duration(c.cfg.GracefulPeriod) * time.Second):
		log.Println("[WARN] Some goroutines will be stopped immediately")
	}

	err = <-c.errChan
	return err
}
