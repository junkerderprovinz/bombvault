package mdns

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/ipv4"
)

var group = &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}

// probeTries bounds how many names the responder tries before it gives up:
// ten instances with the same name on one network is a setup mistake, not a
// race to win.
const probeTries = 10

// iface is one network interface the responder answers on, with the
// addresses it gives out there.
type iface struct {
	ifi   net.Interface
	addrs []netip.Addr
}

// Responder answers multicast DNS queries for one service until Close.
type Responder struct {
	svc    Service
	conn   net.PacketConn
	pc     *ipv4.PacketConn
	ifaces []iface

	// probing is set while the names are being checked; answers wait until
	// they are ours, and conflict records that somebody else has them.
	probing  atomic.Bool
	conflict atomic.Bool

	writeMu  sync.Mutex
	sentMu   sync.Mutex
	lastSent map[string]time.Time

	stop chan struct{}
	done chan struct{}
}

// Start joins the mDNS group on every interface that can carry it, checks
// that the names are free, renaming on a conflict, and announces the service.
func Start(ctx context.Context, svc Service) (*Responder, error) {
	ifaces, err := interfaces()
	if err != nil {
		return nil, err
	}
	lc := net.ListenConfig{Control: reuseAddr}
	conn, err := lc.ListenPacket(ctx, "udp4", "0.0.0.0:5353")
	if err != nil {
		return nil, fmt.Errorf("mdns: listen on port 5353: %w", err)
	}
	pc := ipv4.NewPacketConn(conn)
	var joined []iface
	for _, ifc := range ifaces {
		if err := pc.JoinGroup(&ifc.ifi, group); err != nil {
			log.Printf("mdns: cannot join the multicast group on %s: %v", ifc.ifi.Name, err)
			continue
		}
		joined = append(joined, ifc)
	}
	if len(joined) == 0 {
		_ = conn.Close()
		return nil, errors.New("mdns: no network interface would join the multicast group")
	}
	// Without the receiving interface every answer carries every address,
	// which is still right on a host with one network.
	_ = pc.SetControlMessage(ipv4.FlagInterface, true)
	_ = pc.SetMulticastTTL(255)
	_ = pc.SetMulticastLoopback(true)

	r := &Responder{
		svc:      svc,
		conn:     conn,
		pc:       pc,
		ifaces:   joined,
		lastSent: map[string]time.Time{},
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go r.read()

	if err := r.claim(ctx); err != nil {
		r.shut()
		return nil, err
	}
	r.announce(false)
	go func() {
		select {
		case <-time.After(time.Second):
			r.announce(false)
		case <-r.stop:
		}
	}()
	return r, nil
}

// Service is what the responder announces, with the names it won.
func (r *Responder) Service() Service { return r.svc }

// Close says goodbye, so browsers drop the service at once instead of when
// its records expire, and stops answering.
func (r *Responder) Close() {
	r.announce(true)
	r.shut()
}

func (r *Responder) shut() {
	close(r.stop)
	_ = r.conn.Close()
	<-r.done
}

// claim probes the names three times, 250 ms apart, and takes the next ones
// when anybody answers for them.
func (r *Responder) claim(ctx context.Context) error {
	base := r.svc
	for try := 1; try <= probeTries; try++ {
		if try > 1 {
			r.svc.Instance = base.Instance + " (" + strconv.Itoa(try) + ")"
			r.svc.Host = base.Host + "-" + strconv.Itoa(try)
		}
		r.conflict.Store(false)
		r.probing.Store(true)
		for i := 0; i < 3; i++ {
			for _, ifc := range r.ifaces {
				z, err := r.zone(ifc.addrs)
				if err != nil {
					return err
				}
				msg, err := z.probe()
				if err != nil {
					return err
				}
				r.multicast(ifc, msg)
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
		r.probing.Store(false)
		if !r.conflict.Load() {
			return nil
		}
		log.Printf("mdns: %q or %s.local is taken on this network, trying the next name", r.svc.Instance, r.svc.Host)
	}
	return fmt.Errorf("mdns: every name up to %q is taken on this network", r.svc.Instance)
}

func (r *Responder) zone(addrs []netip.Addr) (zone, error) {
	n, err := r.svc.names()
	if err != nil {
		return zone{}, err
	}
	return zone{names: n, svc: r.svc, addrs: addrs}, nil
}

func (r *Responder) announce(goodbye bool) {
	for _, ifc := range r.ifaces {
		z, err := r.zone(ifc.addrs)
		if err != nil {
			log.Printf("mdns: %v", err)
			return
		}
		msg, err := z.announcement(goodbye)
		if err != nil {
			log.Printf("mdns: build the announcement: %v", err)
			return
		}
		r.multicast(ifc, msg)
	}
}

func (r *Responder) multicast(ifc iface, msg []byte) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if err := r.pc.SetMulticastInterface(&ifc.ifi); err != nil {
		log.Printf("mdns: choose %s for sending: %v", ifc.ifi.Name, err)
		return
	}
	if _, err := r.pc.WriteTo(msg, nil, group); err != nil {
		log.Printf("mdns: send on %s: %v", ifc.ifi.Name, err)
	}
}

// throttled reports whether the same answer went out on this interface less
// than a second ago, the minimum interval RFC 6762 section 6 sets for
// multicast responses.
func (r *Responder) throttled(ifc iface, msg []byte) bool {
	key := ifc.ifi.Name + "|" + string(msg)
	now := time.Now()
	r.sentMu.Lock()
	defer r.sentMu.Unlock()
	if now.Sub(r.lastSent[key]) < time.Second {
		return true
	}
	if len(r.lastSent) > 256 {
		clear(r.lastSent)
	}
	r.lastSent[key] = now
	return false
}

func (r *Responder) read() {
	defer close(r.done)
	buf := make([]byte, 9000)
	for {
		n, cm, src, err := r.pc.ReadFrom(buf)
		if err != nil {
			select {
			case <-r.stop:
				return
			default:
			}
			if errors.Is(err, net.ErrClosed) {
				return
			}
			continue
		}
		msg := buf[:n]
		if r.probing.Load() {
			z, zErr := r.zone(nil)
			if zErr == nil && z.conflicts(msg) {
				r.conflict.Store(true)
			}
			continue
		}
		r.handle(msg, cm, src)
	}
}

func (r *Responder) handle(msg []byte, cm *ipv4.ControlMessage, src net.Addr) {
	ifc, known := r.ifaceOf(cm)
	z, err := r.zone(ifc.addrs)
	if err != nil {
		return
	}
	udp, _ := src.(*net.UDPAddr)
	legacy := udp != nil && udp.Port != group.Port
	resp, err := z.response(msg, legacy)
	if err != nil || resp == nil {
		return
	}
	if legacy {
		r.writeMu.Lock()
		_, _ = r.pc.WriteTo(resp, nil, src)
		r.writeMu.Unlock()
		return
	}
	if !known {
		for _, each := range r.ifaces {
			if !r.throttled(each, resp) {
				r.multicast(each, resp)
			}
		}
		return
	}
	if !r.throttled(ifc, resp) {
		r.multicast(ifc, resp)
	}
}

// ifaceOf is the interface a message came in on. Without the control message
// it is every interface at once, so the answer carries every address.
func (r *Responder) ifaceOf(cm *ipv4.ControlMessage) (iface, bool) {
	if cm != nil {
		for _, ifc := range r.ifaces {
			if ifc.ifi.Index == cm.IfIndex {
				return ifc, true
			}
		}
	}
	var all iface
	for _, ifc := range r.ifaces {
		all.addrs = append(all.addrs, ifc.addrs...)
	}
	return all, false
}

// interfaces lists the interfaces that are up, carry multicast and have an
// IPv4 address other than a link-local one.
func interfaces() ([]iface, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("mdns: list interfaces: %w", err)
	}
	var out []iface
	for _, ifi := range list {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagMulticast == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		var v4 []netip.Addr
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok {
				continue
			}
			ip, ok := netip.AddrFromSlice(ipn.IP)
			if !ok {
				continue
			}
			ip = ip.Unmap()
			if ip.Is4() && !ip.IsLinkLocalUnicast() {
				v4 = append(v4, ip)
			}
		}
		if len(v4) > 0 {
			out = append(out, iface{ifi: ifi, addrs: v4})
		}
	}
	if len(out) == 0 {
		return nil, errors.New("mdns: no network interface with an IPv4 address carries multicast")
	}
	return out, nil
}
