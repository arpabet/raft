/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

// mkCA returns a self-signed CA and its key.
func mkCA(t *testing.T) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             time.Now().Add(-time.Minute),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	cert, _ := x509.ParseCertificate(der)
	return cert, key
}

// mkLeaf signs a node leaf (server+client EKU, SAN IP 127.0.0.1) with the CA.
func mkLeaf(t *testing.T, ca *x509.Certificate, caKey *ecdsa.PrivateKey) tls.Certificate {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "node"},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, ca, &key.PublicKey, caKey)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// The fixed stream layer does real mutual TLS: the Dial side verifies the server
// against the CA and presents its own client cert, and the accepted side (a TLS
// listener, as raft_server.go now wraps it) requires + verifies that client cert.
func TestStreamLayerMutualTLS(t *testing.T) {
	ca, caKey := mkCA(t)
	pool := x509.NewCertPool()
	pool.AddCert(ca)
	mtls := &tls.Config{
		Certificates: []tls.Certificate{mkLeaf(t, ca, caKey)},
		RootCAs:      pool,
		ClientCAs:    pool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	layer := &TCPStreamLayer{listener: tls.NewListener(base, mtls), tlsConfigOpt: mtls}
	defer layer.Close()
	addr := raft.ServerAddress(base.Addr().String())

	peerCert := make(chan bool, 1)
	go func() {
		conn, err := layer.Accept()
		if err != nil {
			peerCert <- false
			return
		}
		defer conn.Close()
		if _, err := conn.Read(make([]byte, 4)); err != nil { // drives the handshake
			peerCert <- false
			return
		}
		peerCert <- len(conn.(*tls.Conn).ConnectionState().PeerCertificates) > 0
	}()

	conn, err := layer.Dial(addr, time.Second)
	if err != nil {
		t.Fatalf("mutual TLS dial failed: %v", err)
	}
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	if !<-peerCert {
		t.Fatal("server did not see a verified client certificate")
	}
	conn.Close()

	// A client presenting no certificate is rejected by RequireAndVerifyClientCert.
	go func() {
		if c, err := layer.Accept(); err == nil {
			c.Read(make([]byte, 1))
			c.Close()
		}
	}()
	noCert := &TCPStreamLayer{tlsConfigOpt: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}
	c, err := noCert.Dial(addr, time.Second)
	if err != nil {
		return // rejected already at dial — acceptable
	}
	c.Write([]byte("x"))
	if _, rerr := c.Read(make([]byte, 1)); rerr == nil {
		t.Fatal("server accepted a client with no certificate")
	}
	c.Close()
}
