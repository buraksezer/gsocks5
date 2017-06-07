package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	uuid "github.com/satori/go.uuid"
)

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
