package main

import (
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/armon/go-socks5"
)

type server struct {
	socksAddr       string
	cfg             config
	keepAlivePeriod time.Duration
	dialTimeout     time.Duration
	wg              sync.WaitGroup
	socks5          *socks5.Server
	errChan         chan error
	signal          chan os.Signal
	done            chan struct{}
}

func newServer(cfg config) *server {
	return &server{
		cfg:             cfg,
		keepAlivePeriod: time.Duration(cfg.KeepAlivePeriod) * time.Second,
		dialTimeout:     time.Duration(cfg.DialTimeout) * time.Second,
		errChan:         make(chan error, 1),
		signal:          make(chan os.Signal),
		done:            make(chan struct{}),
	}
}

func closeConn(conn net.Conn) {
	err := conn.Close()
	if err != nil {
		if opErr, ok := err.(*net.OpError); !ok || (ok && opErr.Op != "accept") {
			log.Println("[ERR] gsocks5: Error while closing socket", conn.RemoteAddr())
		}
	}
}

func (s *server) proxyClientConn(conn, rConn net.Conn, ch chan struct{}) {
	defer s.wg.Done()
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

func (s *server) clientConn(conn net.Conn) {
	defer s.wg.Done()
	defer closeConn(conn)
	rConn, err := net.DialTimeout(s.cfg.Method, s.socksAddr, s.dialTimeout)
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to dial", s.socksAddr, err)
		return
	}
	defer closeConn(rConn)

	// ASSOCIATE command has not been implemented by go-socks5. We currently support TCP but when someone
	// implements ASSOCIATE command, we will implement an UDP relay in gsocks5.
	if s.cfg.Method == "tcp" {
		rConn.(*net.TCPConn).SetKeepAlive(true)
		rConn.(*net.TCPConn).SetKeepAlivePeriod(s.keepAlivePeriod)
	}
	ch := make(chan struct{})
	s.wg.Add(1)
	go s.proxyClientConn(conn, rConn, ch)
	select {
	case <-s.done:
	case <-ch:
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
		log.Println("[ERR] gsocks5: Failed to proxy to ", conn.RemoteAddr())
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

		// ASSOCIATE command has not been implemented by go-socks5. We currently support TCP but when someone
		// implements ASSOCIATE command, we will implement an UDP relay in gsocks5.
		if s.cfg.Method == "tcp" {
			conn.(*net.TCPConn).SetKeepAlive(true)
			conn.(*net.TCPConn).SetKeepAlivePeriod(s.keepAlivePeriod)
		}

		s.wg.Add(1)
		if s.cfg.Role == roleClient {
			go s.clientConn(conn)
			continue
		}
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
	var err error
	var host, port string
	switch {
	case s.cfg.Role == roleClient:
		host, port = s.cfg.ClientHost, s.cfg.ClientPort
	case s.cfg.Role == roleServer:
		host, port = s.cfg.ServerHost, s.cfg.ServerPort
		// Create a SOCKS5 server
		conf := &socks5.Config{}
		s.socks5, err = socks5.New(conf)
		if err != nil {
			return err
		}
	}

	addr := net.JoinHostPort(host, port)
	s.socksAddr = net.JoinHostPort(s.cfg.ServerHost, s.cfg.ServerPort)
	// Create a TCP/UDP server
	l, lerr := net.Listen(s.cfg.Method, addr)
	if lerr != nil {
		return lerr
	}

	s.wg.Add(1)
	go s.serve(l)

	log.Println("[INF] gsocks5: Proxy server runs on", addr)

	select {
	// Wait for SIGINT or SIGTERM
	case <-s.signal:
	// Wait for a listener error
	case <-s.done:
	}

	// Signal all running goroutines to stop.
	log.Println("[INF] gsocks5: Stopping proxy", addr)
	s.shutdown()

	if err = l.Close(); err != nil {
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

	err = <-s.errChan
	return err
}
