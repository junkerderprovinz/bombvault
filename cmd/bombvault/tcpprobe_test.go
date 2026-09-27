package main

import (
	"net"
	"testing"
	"time"
)

func TestTCPProbeAnswersZeroOnceSomethingListens(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	if got := tcpProbe([]string{ln.Addr().String(), "5"}, 10*time.Millisecond); got != 0 {
		t.Fatalf("exit = %d, want 0", got)
	}
}

func TestTCPProbeGivesUpOnAClosedPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	if got := tcpProbe([]string{addr, "1"}, 200*time.Millisecond); got != 1 {
		t.Fatalf("exit = %d, want 1", got)
	}
	if got := tcpProbe([]string{addr}, time.Millisecond); got != 2 {
		t.Fatalf("exit without seconds = %d, want 2", got)
	}
}
