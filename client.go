package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	uuid "github.com/satori/go.uuid"
)

type client struct {
	tcpServerAddr   string
	serverHost      string
	httpClient      *http.Client
	cfg             config
	keepAlivePeriod time.Duration
	dialTimeout     time.Duration
	wg              sync.WaitGroup
	errChan         chan error
	signal          chan os.Signal
	done            chan struct{}
}

func newClient(cfg config, sigChan chan os.Signal) *client {
	tr := &http.Transport{
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.InsecureSkipVerify,
		},
	}
	hc := &http.Client{Transport: tr}
	return &client{
		cfg:             cfg,
		httpClient:      hc,
		keepAlivePeriod: time.Duration(cfg.KeepAlivePeriod) * time.Second,
		dialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		errChan:         make(chan error, 1),
		signal:          sigChan,
		done:            make(chan struct{}),
	}
}

func (c *client) proxyClientConn(conn, rConn net.Conn, ch chan struct{}) {
	defer c.wg.Done()
	defer close(ch)
	var wg sync.WaitGroup
	connCopy := func(dst, src net.Conn) {
		defer wg.Done()
		_, err := io.Copy(dst, src)
		if err != nil {
			if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "readfrom") {
				log.Println("[ERR] gsocks5: Failed to copy connection from",
					src.RemoteAddr(), "to", conn.RemoteAddr(), ":", err)
			}
			return
		}
	}

	wg.Add(2)
	go connCopy(rConn, conn)
	go connCopy(conn, rConn)
	wg.Wait()
}

func (c *client) getConnID() (string, error) {
	endpoint := url.URL{
		Scheme: "https",
		Host:   c.serverHost,
		Path:   newSocksProxyEndpoint,
	}

	resp, err := c.httpClient.Get(endpoint.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	nr := &newProxyRequest{}
	if err = json.NewDecoder(resp.Body).Decode(nr); err != nil {
		log.Println("[ERR] gsocks5: Failed to create a new SOCKS5 proxy, decode:", err)
		return "", err
	}
	return nr.ConnID, nil
}

func (c *client) write(connID string, b []byte) ([]byte, error) {
	endpoint := url.URL{
		Scheme:   "https",
		Host:     c.serverHost,
		Path:     writeSocksProxyEndpoint,
		RawQuery: "connID=" + connID,
	}
	buf := bytes.NewBuffer(b)
	resp, err := c.httpClient.Post(endpoint.String(), "application/octet-stream", buf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// TODO: return a convenient error message
		return nil, errors.New("something went wrong")
	}
	cl := resp.Header.Get("Content-Length")
	l, err := strconv.Atoi(cl)
	if err != nil {
		return nil, err
	}
	body := make([]byte, l)
	_, err = io.ReadFull(resp.Body, body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

// copyBuffer is the actual implementation of Copy and CopyBuffer.
// if buf is nil, one is allocated.
func (c *client) socksOverHTTP(src net.Conn, connID string) error {
	buf := make([]byte, 32*1024)
	type result struct {
		nr  int
		err error
	}

	res := make(chan result, 1)
	rChan := func() chan result {
		nr, er := src.Read(buf)
		rr := result{nr: nr, err: er}
		res <- rr
		return res
	}

	for {
		select {
		case <-c.done:
			return errors.New("[ERR] gsocks5: Request cancelled")
		case res := <-rChan():
			if res.nr > 0 {
				data, ew := c.write(connID, buf[:res.nr])
				if ew != nil {
					return ew
				}
				_, ew = src.Write(data)
				if ew != nil {
					return ew
				}
				nr := len(data)
				if nr >= 6 && nr <= 22 && data[0] == socks5Version && data[1] == socksSuccess {
					return nil
				}
			}
			if res.err != nil {
				if res.err == io.EOF {
					return nil
				}
				return res.err
			}
		}
	}
	return nil
}

func (c *client) clientConn(conn net.Conn) {
	defer c.wg.Done()
	defer closeConn(conn)
	connID, err := c.getConnID()
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to create a new SOCKS5 proxy:", err)
	}
	ch := make(chan struct{})
	if err := c.socksOverHTTP(conn, connID); err != nil {
		log.Println("[ERR] gsocks5: Failed to proxy SOCKS5 over HTTP", err)
		return
	}

	rConn, err := net.DialTimeout(c.cfg.Method, c.tcpServerAddr, c.dialTimeout)
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to dial", c.tcpServerAddr, err)
		return
	}
	defer closeConn(rConn)

	cID, err := uuid.FromString(connID)
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to process ConnID:", connID, err)
		return
	}

	_, err = rConn.Write(cID.Bytes())
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to send ConnID", c.tcpServerAddr, err)
		return
	}

	b := make([]byte, 1)
	_, err = rConn.Read(b)
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to read from raw socket", c.tcpServerAddr, err)
		return
	}

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
			// Shutdown the client immediately.
			c.shutdown()
			if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
				c.errChan <- err
				return
			}
			c.errChan <- nil
			return
		}

		// ASSOCIATE command has not been implemented by go-socks5. We currently support TCP but when someone
		// implements ASSOCIATE command, we will implement an UDP relay in gsocks5.
		if c.cfg.Method == "tcp" {
			conn.(*net.TCPConn).SetKeepAlive(true)
			conn.(*net.TCPConn).SetKeepAlivePeriod(c.keepAlivePeriod)
		}

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
	c.serverHost = net.JoinHostPort(c.cfg.ServerHost, c.cfg.ServerTLSPort)
	c.tcpServerAddr = net.JoinHostPort(c.cfg.ServerHost, c.cfg.ServerPort)

	rawListener, err := net.Listen(c.cfg.Method, addr)
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

	log.Println("[INF] gsocks5: Stopping proxy", addr)
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
