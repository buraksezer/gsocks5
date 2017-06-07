package main

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	uuid "github.com/satori/go.uuid"
)

func (s *server) newSocksProxyHandler(w http.ResponseWriter, req *http.Request) {
	// Send a random UUID as connection ID
	connID := uuid.NewV4()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(connID.Bytes())

	// Create and register a new proxyConn object to manage SOCKS5 over HTTP2
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
	s.connStore.m[conn.connID] = conn

	// Call go-socks5 for handling SOCKS5 protocol.
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
