// Package mdns announces BombVault's web interface on the local network over
// multicast DNS (RFC 6762) with DNS-SD records (RFC 6763), so a browser or a
// service browser finds it by name instead of by address.
package mdns

import (
	"fmt"
	"net/netip"
	"strings"

	"golang.org/x/net/dns/dnsmessage"
)

// TTLs from RFC 6762 section 10: records naming a host get two minutes, the
// rest seventy-five.
const (
	hostTTL  = 120
	otherTTL = 4500
)

// cacheFlush marks a record this responder alone answers for, which tells
// caches to drop what they held for the name.
const cacheFlush = 1 << 15

// Service is what the responder announces.
type Service struct {
	// Instance is the name people see, "BombVault" or "BombVault (tower)".
	Instance string
	// Host is the host label, published as <Host>.local.
	Host string
	// Type is the DNS-SD service type, "_https._tcp" or "_http._tcp".
	Type string
	// Subtype lets a browser ask for BombVault alone among web services.
	Subtype string
	Port    uint16
	TXT     []string
}

// names are the fully qualified names of a service.
type names struct {
	meta, service, subtype, instance, host dnsmessage.Name
}

func (s Service) names() (names, error) {
	instance := strings.ReplaceAll(s.Instance, ".", " ")
	var n names
	for _, p := range []struct {
		dst *dnsmessage.Name
		src string
	}{
		{&n.meta, "_services._dns-sd._udp.local."},
		{&n.service, s.Type + ".local."},
		{&n.subtype, s.Subtype + "._sub." + s.Type + ".local."},
		{&n.instance, instance + "." + s.Type + ".local."},
		{&n.host, s.Host + ".local."},
	} {
		name, err := dnsmessage.NewName(p.src)
		if err != nil {
			return names{}, fmt.Errorf("mdns: name %q: %w", p.src, err)
		}
		*p.dst = name
	}
	return n, nil
}

func sameName(a, b dnsmessage.Name) bool { return strings.EqualFold(a.String(), b.String()) }

// zone is the service with the addresses one interface answers with.
type zone struct {
	names
	svc   Service
	addrs []netip.Addr
}

func (z zone) header(name dnsmessage.Name, typ dnsmessage.Type, ttl uint32, unique bool) dnsmessage.ResourceHeader {
	class := dnsmessage.ClassINET
	if unique {
		class |= cacheFlush
	}
	return dnsmessage.ResourceHeader{Name: name, Type: typ, Class: class, TTL: ttl}
}

func (z zone) ptr(from, to dnsmessage.Name, goodbye bool) dnsmessage.Resource {
	return dnsmessage.Resource{Header: z.header(from, dnsmessage.TypePTR, ttl(otherTTL, goodbye), false), Body: &dnsmessage.PTRResource{PTR: to}}
}

func (z zone) srv(goodbye bool) dnsmessage.Resource {
	return dnsmessage.Resource{
		Header: z.header(z.instance, dnsmessage.TypeSRV, ttl(hostTTL, goodbye), true),
		Body:   &dnsmessage.SRVResource{Port: z.svc.Port, Target: z.host},
	}
}

func (z zone) txt(goodbye bool) dnsmessage.Resource {
	txt := z.svc.TXT
	if len(txt) == 0 {
		txt = []string{""}
	}
	return dnsmessage.Resource{Header: z.header(z.instance, dnsmessage.TypeTXT, ttl(otherTTL, goodbye), true), Body: &dnsmessage.TXTResource{TXT: txt}}
}

func (z zone) a(goodbye bool) []dnsmessage.Resource {
	var out []dnsmessage.Resource
	for _, addr := range z.addrs {
		out = append(out, dnsmessage.Resource{
			Header: z.header(z.host, dnsmessage.TypeA, ttl(hostTTL, goodbye), true),
			Body:   &dnsmessage.AResource{A: addr.As4()},
		})
	}
	return out
}

func ttl(normal uint32, goodbye bool) uint32 {
	if goodbye {
		return 0
	}
	return normal
}

// all is every record of the zone: what an announcement sends, and with a TTL
// of zero what the goodbye sends.
func (z zone) all(goodbye bool) []dnsmessage.Resource {
	out := []dnsmessage.Resource{
		z.ptr(z.meta, z.service, goodbye),
		z.ptr(z.service, z.instance, goodbye),
		z.ptr(z.subtype, z.instance, goodbye),
		z.srv(goodbye),
		z.txt(goodbye),
	}
	return append(out, z.a(goodbye)...)
}

// answer returns the records that answer q and the ones worth sending along,
// both empty when q is about something else.
func (z zone) answer(q dnsmessage.Question) (answers, extra []dnsmessage.Resource) {
	anyType := q.Type == dnsmessage.TypeALL
	want := func(t dnsmessage.Type) bool { return anyType || q.Type == t }
	service := func(from dnsmessage.Name) {
		answers = append(answers, z.ptr(from, z.instance, false))
		extra = append(extra, z.srv(false), z.txt(false))
		extra = append(extra, z.a(false)...)
	}
	switch {
	case sameName(q.Name, z.meta) && want(dnsmessage.TypePTR):
		answers = append(answers, z.ptr(z.meta, z.service, false))
	case sameName(q.Name, z.service) && want(dnsmessage.TypePTR):
		service(z.service)
	case sameName(q.Name, z.subtype) && want(dnsmessage.TypePTR):
		service(z.subtype)
	case sameName(q.Name, z.instance):
		if want(dnsmessage.TypeSRV) {
			answers = append(answers, z.srv(false))
		}
		if want(dnsmessage.TypeTXT) {
			answers = append(answers, z.txt(false))
		}
		if len(answers) > 0 {
			extra = append(extra, z.a(false)...)
		}
	case sameName(q.Name, z.host) && want(dnsmessage.TypeA):
		answers = append(answers, z.a(false)...)
	}
	return answers, extra
}

// response answers a query, or returns nil when none of its questions is
// about this zone. A query from a port other than 5353 comes from a simple
// resolver that waits on its own socket: it gets its id and questions back and
// no cache-flush bits, as RFC 6762 section 6.7 asks.
func (z zone) response(query []byte, legacy bool) ([]byte, error) {
	var p dnsmessage.Parser
	h, err := p.Start(query)
	if err != nil || h.Response {
		return nil, err
	}
	questions, err := p.AllQuestions()
	if err != nil {
		return nil, err
	}
	var answers, extra []dnsmessage.Resource
	for _, q := range questions {
		a, e := z.answer(q)
		answers = append(answers, a...)
		extra = append(extra, e...)
	}
	if len(answers) == 0 {
		return nil, nil
	}
	head := dnsmessage.Header{Response: true, Authoritative: true}
	var echo []dnsmessage.Question
	if legacy {
		head.ID = h.ID
		echo = questions
		for i := range answers {
			answers[i].Header.Class &^= cacheFlush
		}
		for i := range extra {
			extra[i].Header.Class &^= cacheFlush
		}
	}
	return build(head, echo, answers, nil, dedupe(answers, extra))
}

// dedupe drops the extra records the answer section already carries.
func dedupe(answers, extra []dnsmessage.Resource) []dnsmessage.Resource {
	seen := map[string]bool{}
	for _, r := range answers {
		seen[r.GoString()] = true
	}
	var out []dnsmessage.Resource
	for _, r := range extra {
		if !seen[r.GoString()] {
			seen[r.GoString()] = true
			out = append(out, r)
		}
	}
	return out
}

// announcement is the unsolicited response that tells the network about the
// zone, or with goodbye set that it is gone.
func (z zone) announcement(goodbye bool) ([]byte, error) {
	return build(dnsmessage.Header{Response: true, Authoritative: true}, nil, z.all(goodbye), nil, nil)
}

// probe asks whether anybody else already answers for the zone's names, with
// the records it wants to claim in the authority section (RFC 6762 section 8.1).
func (z zone) probe() ([]byte, error) {
	questions := []dnsmessage.Question{
		{Name: z.instance, Type: dnsmessage.TypeALL, Class: dnsmessage.ClassINET | cacheFlush},
		{Name: z.host, Type: dnsmessage.TypeALL, Class: dnsmessage.ClassINET | cacheFlush},
	}
	authority := append([]dnsmessage.Resource{z.srv(false), z.txt(false)}, z.a(false)...)
	for i := range authority {
		authority[i].Header.Class &^= cacheFlush
	}
	return build(dnsmessage.Header{}, questions, nil, authority, nil)
}

// conflicts reports whether msg is a response from somebody else that
// answers for one of the zone's unique names.
func (z zone) conflicts(msg []byte) bool {
	var p dnsmessage.Parser
	h, err := p.Start(msg)
	if err != nil || !h.Response {
		return false
	}
	if err := p.SkipAllQuestions(); err != nil {
		return false
	}
	answers, err := p.AllAnswers()
	if err != nil {
		return false
	}
	for _, r := range answers {
		if sameName(r.Header.Name, z.instance) || sameName(r.Header.Name, z.host) {
			return true
		}
	}
	return false
}

func build(h dnsmessage.Header, questions []dnsmessage.Question, answers, authority, extra []dnsmessage.Resource) ([]byte, error) {
	b := dnsmessage.NewBuilder(make([]byte, 0, 512), h)
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, err
	}
	for _, q := range questions {
		if err := b.Question(q); err != nil {
			return nil, err
		}
	}
	sections := []struct {
		start func() error
		rs    []dnsmessage.Resource
	}{
		{b.StartAnswers, answers},
		{b.StartAuthorities, authority},
		{b.StartAdditionals, extra},
	}
	for _, s := range sections {
		if err := s.start(); err != nil {
			return nil, err
		}
		for _, r := range s.rs {
			if err := add(&b, r); err != nil {
				return nil, err
			}
		}
	}
	return b.Finish()
}

func add(b *dnsmessage.Builder, r dnsmessage.Resource) error {
	switch body := r.Body.(type) {
	case *dnsmessage.PTRResource:
		return b.PTRResource(r.Header, *body)
	case *dnsmessage.SRVResource:
		return b.SRVResource(r.Header, *body)
	case *dnsmessage.TXTResource:
		return b.TXTResource(r.Header, *body)
	case *dnsmessage.AResource:
		return b.AResource(r.Header, *body)
	}
	return fmt.Errorf("mdns: cannot pack %T", r.Body)
}
