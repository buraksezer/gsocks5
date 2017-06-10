package main

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"github.com/armon/go-socks5"
)

var authSuccess = []byte{1}
var errAuthenticationFailed = errors.New("authentication failed")

type server struct {
	cfg      config
	password []byte
	wg       sync.WaitGroup
	socks5   *socks5.Server
	errChan  chan error
	signal   chan os.Signal
	done     chan struct{}
}

func newServer(cfg config, sigChan chan os.Signal) *server {
	return &server{
		cfg:     cfg,
		signal:  sigChan,
		errChan: make(chan error, 2),
		done:    make(chan struct{}),
	}
}

func (s *server) authenticate(conn net.Conn, errChan chan error) {
	defer s.wg.Done()
	buf := make([]byte, maxPasswordLength)
	nr, err := conn.Read(buf)
	if err == io.EOF {
		err = nil
	}
	if err != nil {
		errChan <- err
		return
	}
	if !bytes.Equal(buf[:nr], s.password) {
		errChan <- errAuthenticationFailed
		return
	}
	_, err = conn.Write(authSuccess)
	if err != nil {
		log.Println("[ERR] gsocks5: Failed to write authSuccess message", conn.RemoteAddr(), ":", err)
		errChan <- err
		return
	}
	errChan <- nil
	return
}

func (s *server) closeConnAtBackground(conn net.Conn, ch chan struct{}) {
	defer s.wg.Done()
	select {
	case <-s.done:
	case <-ch:
	}
	closeConn(conn)
}

func (s *server) connSocks5(conn net.Conn) {
	defer s.wg.Done()
	ch := make(chan struct{})
	defer close(ch)

	s.wg.Add(1)
	go s.closeConnAtBackground(conn, ch)

	if s.password != nil {
		errChan := make(chan error, 1)
		s.wg.Add(1)
		go s.authenticate(conn, errChan)
		select {
		case <-time.After(5 * time.Second):
			log.Println("[ERR] gsocks5: Authentication expired", conn.RemoteAddr())
			return
		case err := <-errChan:
			if err != nil {
				log.Println("[ERR] gsocks5: Failed to auth", conn.RemoteAddr(), ":", err)
				return
			}
		}
		log.Println("[DEBUG] gsocks5: Authenticated TCP connection from", conn.RemoteAddr())
	}
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
	if s.cfg.Password != "" {
		s.password = []byte(s.cfg.Password)
		if len(s.password) > maxPasswordLength {
			return errPasswordTooLong
		}
	}

	// Create a SOCKS5 server
	conf := &socks5.Config{}
	if s.cfg.Socks5Password != "" && s.cfg.Socks5Username != "" {
		creds := socks5.StaticCredentials{
			s.cfg.Socks5Username: s.cfg.Socks5Password,
		}
		cator := socks5.UserPassAuthenticator{Credentials: creds}
		conf.AuthMethods = []socks5.Authenticator{cator}
	}
	ss, err := socks5.New(conf)
	if err != nil {
		return err
	}
	s.socks5 = ss

	cer, err := tls.LoadX509KeyPair(s.cfg.ServerCert, s.cfg.ServerKey)
	if err != nil {
		return err
	}
	config := &tls.Config{Certificates: []tls.Certificate{cer}}
	ln, err := tls.Listen("tcp", s.cfg.ServerAddr, config)
	if err != nil {
		return err
	}

	log.Println("[INF] gsocks5: TLS server runs on", s.cfg.ServerAddr)
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

	log.Println("[INF] gsocks5: Stopping proxy", s.cfg.ServerAddr)
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
