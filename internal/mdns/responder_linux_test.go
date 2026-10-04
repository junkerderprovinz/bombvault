package mdns

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestTheResponderAnswersASimpleResolver(t *testing.T) {
	buf := make([]byte, 4)
	_, _ = rand.Read(buf)
	host := "bvtest-" + hex.EncodeToString(buf)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	r, err := Start(ctx, Service{Instance: "BombVault test", Host: host, Type: "_https._tcp", Subtype: "_bombvault", Port: 3443})
	if err != nil {
		t.Skipf("no multicast network here: %v", err)
	}
	defer r.Close()

	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := conn.WriteToUDP(query(t, 7, host+".local.", dnsmessage.TypeA), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 5353}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	reply := make([]byte, 1500)
	n, _, err := conn.ReadFromUDP(reply)
	if err != nil {
		t.Fatalf("no reply: %v", err)
	}
	got := parse(t, reply[:n])
	if got.header.ID != 7 || len(got.answers) == 0 {
		t.Fatalf("reply %+v with %d answers", got.header, len(got.answers))
	}
	if _, ok := got.answers[0].Body.(*dnsmessage.AResource); !ok {
		t.Fatalf("answer %v, want an A record", got.answers[0])
	}
}
