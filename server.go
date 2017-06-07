package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/armon/go-socks5"
	uuid "github.com/satori/go.uuid"
)

type server struct {
	cfg             config
	httpServer      *http.Server
	connStore       *connStore
	keepAlivePeriod time.Duration
	wg              sync.WaitGroup
	socks5          *socks5.Server
	errChan         chan error
	signal          chan os.Signal
	done            chan struct{}
}

func newServer(cfg config, sigChan chan os.Signal) *server {
	cs := &connStore{
		m: make(map[string]*proxyConn),
	}
	return &server{
		cfg:             cfg,
		connStore:       cs,
		keepAlivePeriod: time.Duration(cfg.KeepAlivePeriod) * time.Second,
		errChan:         make(chan error, 2),
		signal:          sigChan,
		done:            make(chan struct{}),
	}
}

func (s *server) connSocks5(conn net.Conn) {
	defer s.wg.Done()
	ch := make(chan struct{})
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		select {
		case <-s.done:
			// Close the connection immediately. The process is shutting down.
			closeConn(conn)
		case <-ch:
		}
	}()

	defer close(ch)
	if err := s.socks5.ServeConn(conn); err != nil {
		log.Println("[ERR] gsocks5: Failed to proxy to ", conn.RemoteAddr(), err)
	}
}

func (s *server) findSocksSocket(conn net.Conn) {
	defer s.wg.Done()
	buf := make([]byte, 20)
	nr, err := conn.Read(buf)
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to proxy to", conn.RemoteAddr(), err)
	}
	connID, err := uuid.FromBytes(buf[:nr])
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to proxy to", conn.RemoteAddr(), err)
	}
	s.connStore.mu.Lock()
	defer s.connStore.mu.Unlock()
	c, ok := s.connStore.m[connID.String()]
	if !ok {
		log.Println("[ERR] gsocks5: Failed to proxy, not found", connID)
		return
	}
	b := []byte{0}
	conn.Write(b)
	c.Conn = conn
	close(c.connReady)
}

func (s *server) serve(l net.Listener) {
	defer s.wg.Done()
	for {
		conn, err := l.Accept()
		if err != nil {
			// Shutdown the server immediately.
			s.shutdown()
			if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
				s.errChan <- err
				return
			}
			s.errChan <- nil
			return
		}
		s.wg.Add(1)
		go s.findSocksSocket(conn)
	}
}

func (s *server) shutdown() {
	select {
	case <-s.done:
		return
	default:
	}
	close(s.done)
}

const (
	newSocksProxyEndpoint   = "/new-socks-proxy"
	writeSocksProxyEndpoint = "/write-socks-proxy"
)

func (s *server) run() error {
	host, port := s.cfg.ServerHost, s.cfg.ServerPort
	// Create a SOCKS5 server
	conf := &socks5.Config{}
	ss, err := socks5.New(conf)
	if err != nil {
		return err
	}
	s.socks5 = ss

	addr := net.JoinHostPort(host, port)
	rawListener, err := net.Listen(s.cfg.Method, addr)
	if err != nil {
		return err
	}

	log.Println("[INF] gsocks5: Proxy server runs on", addr)
	s.wg.Add(1)
	go s.serve(rawListener)

	mux := http.NewServeMux()
	s.httpServer = &http.Server{
		Handler: mux,
		Addr:    net.JoinHostPort(s.cfg.ServerHost, s.cfg.ServerTLSPort),
	}
	mux.HandleFunc(newSocksProxyEndpoint, s.newSocksProxyHandler)
	mux.HandleFunc(writeSocksProxyEndpoint, s.writeSocksProxyHandler)

	httpErr := make(chan error, 1)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		log.Println("[INF] gsocks5: HTTP/2 server runs on", s.httpServer.Addr)
		httpErr <- s.httpServer.ListenAndServeTLS(s.cfg.ServerCert, s.cfg.ServerKey)
	}()

	select {
	// Wait for SIGINT or SIGTERM
	case <-s.signal:
	// Wait for a listener error
	case <-s.done:
	case hErr := <-httpErr:
		log.Println("[ERR] gsocks5: Failed to listen HTTPS", hErr)

	}

	// Signal all running goroutines to stop.
	s.shutdown()

	// TODO: Check CancelFunc
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	// Shutdown HTTP server
	s.httpServer.Shutdown(ctx)

	log.Println("[INF] gsocks5: Stopping proxy", addr)
	if err = rawListener.Close(); err != nil {
		log.Println("[ERR] gsocks5: Failed to close listener", err)
	}

	ch := make(chan struct{})
	go func() {
		defer close(ch)
		s.wg.Wait()
	}()

	select {
	case <-ch:
	case <-time.After(time.Duration(s.cfg.GracefulPeriod) * time.Second):
		log.Println("[WARN] Some goroutines will be stopped immediately")
	}

	return <-s.errChan
}
