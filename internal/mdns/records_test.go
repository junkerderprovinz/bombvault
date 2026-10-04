package mdns

import (
	"net/netip"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

func testZone(t *testing.T) zone {
	t.Helper()
	svc := Service{
		Instance: "BombVault (tower.lan)",
		Host:     "bombvault",
		Type:     "_https._tcp",
		Subtype:  "_bombvault",
		Port:     3443,
		TXT:      []string{"version=v9.1.0", "path=/"},
	}
	n, err := svc.names()
	if err != nil {
		t.Fatal(err)
	}
	return zone{names: n, svc: svc, addrs: []netip.Addr{netip.MustParseAddr("192.168.1.20")}}
}

func query(t *testing.T, id uint16, name string, typ dnsmessage.Type) []byte {
	t.Helper()
	msg, err := build(dnsmessage.Header{ID: id}, []dnsmessage.Question{{
		Name: dnsmessage.MustNewName(name), Type: typ, Class: dnsmessage.ClassINET,
	}}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	return msg
}

type parsed struct {
	header      dnsmessage.Header
	questions   []dnsmessage.Question
	answers     []dnsmessage.Resource
	additionals []dnsmessage.Resource
}

func parse(t *testing.T, msg []byte) parsed {
	t.Helper()
	var p dnsmessage.Parser
	h, err := p.Start(msg)
	if err != nil {
		t.Fatal(err)
	}
	out := parsed{header: h}
	if out.questions, err = p.AllQuestions(); err != nil {
		t.Fatal(err)
	}
	if out.answers, err = p.AllAnswers(); err != nil {
		t.Fatal(err)
	}
	if err = p.SkipAllAuthorities(); err != nil {
		t.Fatal(err)
	}
	if out.additionals, err = p.AllAdditionals(); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestABrowserFindsTheWebInterface(t *testing.T) {
	z := testZone(t)
	for _, name := range []string{"_https._tcp.local.", "_bombvault._sub._https._tcp.local.", "_HTTPS._tcp.local."} {
		resp, err := z.response(query(t, 0, name, dnsmessage.TypePTR), false)
		if err != nil || resp == nil {
			t.Fatalf("%s: no answer (%v)", name, err)
		}
		got := parse(t, resp)
		if !got.header.Response || !got.header.Authoritative {
			t.Fatalf("%s: header %+v", name, got.header)
		}
		ptr, ok := got.answers[0].Body.(*dnsmessage.PTRResource)
		if len(got.answers) != 1 || !ok || ptr.PTR.String() != "BombVault (tower lan)._https._tcp.local." {
			t.Fatalf("%s: answers %v", name, got.answers)
		}
		var port uint16
		var addr [4]byte
		var txt []string
		for _, r := range got.additionals {
			if r.Header.Class&cacheFlush == 0 {
				t.Errorf("%s: %v lacks the cache-flush bit", name, r.Header)
			}
			switch b := r.Body.(type) {
			case *dnsmessage.SRVResource:
				port = b.Port
				if b.Target.String() != "bombvault.local." {
					t.Errorf("SRV target %s", b.Target)
				}
			case *dnsmessage.AResource:
				addr = b.A
			case *dnsmessage.TXTResource:
				txt = b.TXT
			}
		}
		if port != 3443 || addr != [4]byte{192, 168, 1, 20} || len(txt) != 2 || txt[0] != "version=v9.1.0" || txt[1] != "path=/" {
			t.Fatalf("%s: port %d, address %v, txt %v", name, port, addr, txt)
		}
	}
}

func TestTheHostNameResolves(t *testing.T) {
	z := testZone(t)
	resp, err := z.response(query(t, 0, "bombvault.local.", dnsmessage.TypeA), false)
	if err != nil || resp == nil {
		t.Fatalf("no answer (%v)", err)
	}
	got := parse(t, resp)
	a, ok := got.answers[0].Body.(*dnsmessage.AResource)
	if !ok || a.A != [4]byte{192, 168, 1, 20} || got.answers[0].Header.TTL != hostTTL {
		t.Fatalf("answers %v", got.answers)
	}
}

func TestASimpleResolverGetsItsQuestionBack(t *testing.T) {
	z := testZone(t)
	resp, err := z.response(query(t, 0x4d2, "bombvault.local.", dnsmessage.TypeA), true)
	if err != nil || resp == nil {
		t.Fatalf("no answer (%v)", err)
	}
	got := parse(t, resp)
	if got.header.ID != 0x4d2 || len(got.questions) != 1 {
		t.Fatalf("header %+v, questions %v", got.header, got.questions)
	}
	if got.answers[0].Header.Class != dnsmessage.ClassINET {
		t.Fatalf("a unicast answer carries class %v, want plain IN", got.answers[0].Header.Class)
	}
}

func TestOtherNamesGetNoAnswer(t *testing.T) {
	z := testZone(t)
	for _, q := range []struct {
		name string
		typ  dnsmessage.Type
	}{
		{"_http._tcp.local.", dnsmessage.TypePTR},
		{"tower.local.", dnsmessage.TypeA},
		{"bombvault.local.", dnsmessage.TypeAAAA},
	} {
		resp, err := z.response(query(t, 0, q.name, q.typ), false)
		if err != nil || resp != nil {
			t.Fatalf("%s %v answered %v (%v)", q.name, q.typ, resp, err)
		}
	}
}

func TestGoodbyeSetsEveryTTLToZero(t *testing.T) {
	z := testZone(t)
	msg, err := z.announcement(true)
	if err != nil {
		t.Fatal(err)
	}
	got := parse(t, msg)
	if len(got.answers) != 6 {
		t.Fatalf("%d records, want three PTRs, SRV, TXT and A", len(got.answers))
	}
	for _, r := range got.answers {
		if r.Header.TTL != 0 {
			t.Fatalf("%v keeps TTL %d", r.Header.Name, r.Header.TTL)
		}
	}
}

func TestAnotherAnswerForOurNamesIsAConflict(t *testing.T) {
	z := testZone(t)
	theirs := z
	theirs.addrs = []netip.Addr{netip.MustParseAddr("192.168.1.30")}
	announce, err := theirs.announcement(false)
	if err != nil {
		t.Fatal(err)
	}
	if !z.conflicts(announce) {
		t.Fatal("another host answering for our names is not a conflict")
	}
	probe, err := z.probe()
	if err != nil {
		t.Fatal(err)
	}
	if z.conflicts(probe) {
		t.Fatal("a probe, which is a query, counts as a conflict")
	}
	other := testZone(t)
	other.svc.Host = "nas"
	other.svc.Instance = "NAS"
	if other.names, err = other.svc.names(); err != nil {
		t.Fatal(err)
	}
	unrelated, err := other.announcement(false)
	if err != nil {
		t.Fatal(err)
	}
	if z.conflicts(unrelated) {
		t.Fatal("an unrelated service counts as a conflict")
	}
}
