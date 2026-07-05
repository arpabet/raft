/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"crypto/tls"
	"net"
	"time"

	"github.com/hashicorp/raft"
	"golang.org/x/xerrors"
)

var (
	errNotAdvertisable = xerrors.New("local bind address is not advertisable")
	errNotTCP          = xerrors.New("local address is not a TCP address")
)

// TCPStreamLayer implements StreamLayer interface for plain TCP.
type TCPStreamLayer struct {
	advertise    net.Addr
	listener     net.Listener
	tlsConfigOpt *tls.Config // can be nil
}

func newTCPTransport(listener net.Listener,
	advertise net.Addr,
	tlsConfigOpt *tls.Config, // can be nil
	transportCreator func(stream raft.StreamLayer) *raft.NetworkTransport) (*raft.NetworkTransport, error) {

	// Create stream
	stream := &TCPStreamLayer{
		advertise:    advertise,
		listener:     listener,
		tlsConfigOpt: tlsConfigOpt,
	}

	// Verify that we have a usable advertise address
	addr, ok := stream.Addr().(*net.TCPAddr)
	if !ok {
		return nil, errNotTCP
	}
	if addr.IP == nil || addr.IP.IsUnspecified() {
		return nil, errNotAdvertisable
	}

	// Create the network transport
	trans := transportCreator(stream)
	return trans, nil
}

// Dial implements the StreamLayer interface.
func (t *TCPStreamLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {

	if t.tlsConfigOpt != nil {

		// Verify the peer server against our CA (RootCAs) rather than skipping
		// verification: this is real mutual TLS. tls.DialWithDialer derives the
		// ServerName from the dial address host when the config leaves it empty, so
		// it is matched against the peer certificate's SANs (the node's advertised
		// IP / id). We present our own client certificate from Certificates.
		tlsConf := t.tlsConfigOpt.Clone()

		d := net.Dialer{Timeout: timeout}
		return tls.DialWithDialer(&d, "tcp", string(address), tlsConf)
	} else {
		return net.DialTimeout("tcp", string(address), timeout)
	}

}

// Accept implements the net.Listener interface.
func (t *TCPStreamLayer) Accept() (c net.Conn, err error) {
	return t.listener.Accept()
}

// Close implements the net.Listener interface.
func (t *TCPStreamLayer) Close() (err error) {
	return t.listener.Close()
}

// Addr implements the net.Listener interface.
func (t *TCPStreamLayer) Addr() net.Addr {
	// Use an advertise addr if provided
	if t.advertise != nil {
		return t.advertise
	}
	return t.listener.Addr()
}
