package main

import "net"

func listenAddr(listen, port string) string {
	if listen != "" {
		return listen
	}
	return net.JoinHostPort("", port)
}
