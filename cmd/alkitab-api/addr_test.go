package main

import "testing"

func TestListenAddr(t *testing.T) {
	if got := listenAddr("", "3000"); got != ":3000" {
		t.Errorf("default: %q", got)
	}
	if got := listenAddr("127.0.0.1:8080", "3000"); got != "127.0.0.1:8080" {
		t.Errorf("override: %q", got)
	}
}
