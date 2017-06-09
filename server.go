package main

import (
	"crypto/tls"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/armon/go-socks5"
)

type server struct {
	cfg             config
	keepAlivePeriod time.Duration
	wg              sync.WaitGroup
	socks5          *socks5.Server
	errChan         chan error
	signal          chan os.Signal
	done            chan struct{}
}

func newServer(cfg config, sigChan chan os.Signal) *server {
	return &server{
		cfg:             cfg,
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
	go func(conn net.Conn) {
		defer s.wg.Done()
		select {
		case <-s.done:
			// Close the connection immediately. The process is shutting down.
			closeConn(conn)
		case <-ch:
		}
	}(conn)

	defer close(ch)
	if err := s.socks5.ServeConn(conn); err != nil {
		opErr, ok := err.(*net.OpError)
		switch {
		case ok && opErr.Op == "readfrom":
			return
		case ok && opErr.Op == "read":
			return
		default:
		}
		log.Println("[ERR] gsocks5: Failed to proxy", conn.RemoteAddr(), ":", err)
	}
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
		go s.connSocks5(conn)
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
	cer, err := tls.LoadX509KeyPair(s.cfg.ServerCert, s.cfg.ServerKey)
	if err != nil {
		return err
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	ln, err := tls.Listen("tcp", addr, config)
	if err != nil {
		return err
	}

	log.Println("[INF] gsocks5: TLS server runs on", addr)
	s.wg.Add(1)
	go s.serve(ln)

	select {
	// Wait for SIGINT or SIGTERM
	case <-s.signal:
	// Wait for a listener error
	case <-s.done:
	}

	// Signal all running goroutines to stop.
	s.shutdown()

	log.Println("[INF] gsocks5: Stopping proxy", addr)
	if err = ln.Close(); err != nil {
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
