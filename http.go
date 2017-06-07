package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"

	uuid "github.com/satori/go.uuid"
)

type proxyConn struct {
	net.Conn
	connStore  *connStore
	buf        *bytes.Reader
	response   chan []byte
	incoming   chan *bytes.Reader
	connReady  chan struct{}
	httpDone   chan struct{}
	remoteAddr string
	connID     string
}

func (pc *proxyConn) getConn() net.Conn {
	<-pc.connReady
	return pc.Conn
}

func (pc *proxyConn) Close() error {
	pc.connStore.mu.Lock()
	delete(pc.connStore.m, pc.connID)
	pc.connStore.mu.Unlock()

	select {
	case <-pc.connReady:
		return pc.Conn.Close()
	default:
	}
	// There is nothing to do
	return nil
}

func (pc *proxyConn) RemoteAddr() net.Addr {
	select {
	case <-pc.connReady:
		// Raw TCP socket
		return pc.Conn.RemoteAddr()
	default:
	}
	// HTTP socket.
	ip, port, _ := net.SplitHostPort(pc.remoteAddr)
	p, _ := strconv.Atoi(port)
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: p}
}

func (pc *proxyConn) read(b []byte) (int, error) {
	conn := pc.getConn()
	return conn.Read(b)
}

func (pc *proxyConn) write(b []byte) (int, error) {
	conn := pc.getConn()
	return conn.Write(b)
}

func (pc *proxyConn) Read(b []byte) (int, error) {
	select {
	case <-pc.httpDone:
		return pc.read(b)
	default:
	}

	buf := <-pc.incoming
	nr, err := buf.Read(b)
	if err == nil {
		pc.incoming <- buf
	}
	if err == io.EOF {
		err = nil
	}
	return nr, err
}

func (pc *proxyConn) Write(b []byte) (int, error) {
	select {
	case <-pc.httpDone:
		return pc.write(b)
	default:
	}

	// TODO: Explain that shit.
	nr := len(b)
	if nr >= 6 && nr <= 22 && b[0] == socks5Version && b[1] == socksSuccess {
		close(pc.httpDone)
	}

	pc.response <- b
	return len(b), nil
}

type connStore struct {
	mu sync.RWMutex
	m  map[string]*proxyConn
}

type newProxyRequest struct {
	ConnID string `json:"conn_id"`
}

func (s *server) newSocksProxyHandler(w http.ResponseWriter, req *http.Request) {
	connID := uuid.NewV4()
	n := newProxyRequest{
		ConnID: connID.String(),
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(n); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	s.connStore.mu.Lock()
	defer s.connStore.mu.Unlock()
	conn := &proxyConn{
		connStore:  s.connStore,
		connID:     connID.String(),
		remoteAddr: req.RemoteAddr,
		connReady:  make(chan struct{}),
		response:   make(chan []byte, 1),
		incoming:   make(chan *bytes.Reader, 1),
		httpDone:   make(chan struct{}),
	}
	s.connStore.m[n.ConnID] = conn
	s.wg.Add(1)
	go s.connSocks5(conn)
}

func (s *server) writeSocksProxyHandler(w http.ResponseWriter, req *http.Request) {
	s.connStore.mu.Lock()
	defer s.connStore.mu.Unlock()

	connID := req.URL.Query().Get("connID")
	pConn, ok := s.connStore.m[connID]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
	}
	cl := req.Header.Get("Content-Length")
	l, err := strconv.Atoi(cl)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
	body := make([]byte, l)
	_, err = io.ReadFull(req.Body, body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	pConn.incoming <- bytes.NewReader(body)
	data := <-pConn.response
	w.Write(data)
}
