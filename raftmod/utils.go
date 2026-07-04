/*
 * Copyright (c) 2025-2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package raftmod

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"go.arpabet.com/cligo"
	"go.arpabet.com/servion"
	"golang.org/x/xerrors"
)

func panicToError(err *error) {
	if r := recover(); r != nil {
		switch v := r.(type) {
		case error:
			*err = v
		case string:
			*err = xerrors.New(v)
		default:
			*err = xerrors.Errorf("%v", v)
		}
	}
}

func getPortNumber(address string) (int, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return 0, xerrors.Errorf("empty port in address '%s', %v", address, err)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return 0, xerrors.Errorf("invalid port number in address '%s', %v", address, err)
	}
	return portNum, nil
}

func getHostAndPortNumber(address string) (string, int, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", 0, xerrors.Errorf("empty port in address '%s', %v", address, err)
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, xerrors.Errorf("invalid port number in address '%s', %v", address, err)
	}
	return host, portNum, err
}

func createDirIfNeeded(dir string, perm os.FileMode) error {
	if _, err := os.Stat(dir); err != nil {
		if err = os.Mkdir(dir, perm); err != nil {
			return xerrors.Errorf("unable to create dir '%s' with permissions %x, %v", dir, perm, err)
		}
		if err = os.Chmod(dir, perm); err != nil {
			return xerrors.Errorf("unable to chmod dir '%s' with permissions %x, %v", dir, perm, err)
		}
	}
	return nil
}

// PrivateIP get the host machine private IP address
func PrivateIP() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip.IsPrivate() {
				return ip, nil
			}

		}
	}

	return nil, xerrors.New("no IP")
}

func GetIP(addr net.Addr) []byte {
	switch a := addr.(type) {
	case *net.UDPAddr:
		return []byte(a.IP.String())
	case *net.TCPAddr:
		return []byte(a.IP.String())
	}
	return []byte{}
}

func addLocalIP(addr string) string {
	parts := strings.Split(addr, ":")
	if parts[0] == "" {
		ipAddr, err := PrivateIP()
		if err == nil {
			parts[0] = ipAddr.String()
			return strings.Join(parts, ":")
		}
	}
	return addr
}

func ReplaceToPrivateIP(addr string) string {
	parts := strings.Split(addr, ":")
	if parts[0] == "" || parts[0] == "0.0.0.0" || parts[0] == "127.0.0.1" {
		ipAddr, err := PrivateIP()
		if err == nil {
			parts[0] = ipAddr.String()
			return strings.Join(parts, ":")
		}
	}
	return addr
}

func ParseAndAdjustTCPAddr(address string, seq int) (*net.TCPAddr, error) {

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, xerrors.Errorf("empty port in address '%s', %v", address, err)
	}
	if host == "" {
		// empty host means all IPs
		host = "0.0.0.0"
	}

	addr := fmt.Sprintf("%s:%s", host, port)

	// Resolve the address
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return nil, xerrors.Errorf("invalid address '%s', %v", addr, err)
	}

	tcpAddr.Port += seq

	return tcpAddr, nil

}

/**
Resolves the application name: explicit 'application.name' property first,
then the cligo application bean, then "raft".
*/
func resolveApplicationName(prop string, cliApp cligo.CliApplication) string {
	if prop != "" {
		return prop
	}
	if cliApp != nil {
		return cliApp.Name()
	}
	return "raft"
}

/**
Maps a dotted property key to an environment variable name,
e.g. raft.snapshot-key -> RAFT_SNAPSHOT_KEY.
*/
func envKey(key string) string {
	b := make([]byte, len(key))
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= 'a' && c <= 'z':
			b[i] = c - 'a' + 'A'
		case (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'):
			b[i] = c
		default:
			b[i] = '_'
		}
	}
	return string(b)
}

/**
Home directory of the application: the servion runtime home when running
under servion, otherwise the current directory.
*/
func homeDir(runtime servion.Runtime) string {
	if runtime != nil {
		return runtime.HomeDir()
	}
	return "."
}
