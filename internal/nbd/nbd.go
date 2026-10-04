// Package nbd is a small client for the NBD protocol, enough to read a disk
// that libvirt exports during a pull-mode backup: fixed newstyle handshake,
// structured replies, meta contexts, block status and reads. It keeps one
// request in flight at a time.
package nbd

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
)

const (
	magicInit      = 0x4e42444d41474943 // "NBDMAGIC"
	magicOpt       = 0x49484156454f5054 // "IHAVEOPT"
	magicOptReply  = 0x0003e889045565a9
	magicRequest   = 0x25609513
	magicSimple    = 0x67446698
	magicStructure = 0x668e33ef

	flagFixedNewstyle = 1 << 0
	flagNoZeroes      = 1 << 1

	optGo              = 7
	optStructuredReply = 8
	optSetMetaContext  = 10

	repAck         = 1
	repInfo        = 3
	repMetaContext = 4
	repErrBit      = 1 << 31

	infoExport = 0

	cmdRead        = 0
	cmdDisc        = 2
	cmdBlockStatus = 7

	replyFlagDone = 1 << 0

	replyNone        = 0
	replyOffsetData  = 1
	replyOffsetHole  = 2
	replyBlockStatus = 5
	replyErrBit      = 1 << 15

	// maxRead stays under the 32 MiB qemu accepts in one request.
	maxRead = 16 << 20
	// maxStatus is how much one block status request asks about; the server
	// may answer for less and the loop asks again.
	maxStatus = 1 << 30
)

// Extent flags. For the "base:allocation" context StateHole means no data is
// allocated and StateZero that the range reads as zeros. For a
// "qemu:dirty-bitmap:" context StateDirty means the range changed.
const (
	StateHole  = 1 << 0
	StateZero  = 1 << 1
	StateDirty = 1 << 0
)

// AllocationContext is the standard meta context that reports holes and
// zero ranges.
const AllocationContext = "base:allocation"

// Extent is one run of the export with the flags a meta context reported.
type Extent struct {
	Offset int64
	Length int64
	Flags  uint32
}

// Client is a connection to one NBD export.
type Client struct {
	mu       sync.Mutex
	conn     io.ReadWriteCloser
	size     int64
	contexts map[string]uint32
	cookie   uint64
}

// Dial connects to the export on a Unix socket or TCP address and negotiates
// the given meta contexts. A context the server does not know is left out, so
// a caller checks HasContext before relying on it.
func Dial(ctx context.Context, network, address, export string, contexts []string) (*Client, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("nbd: connect: %w", err)
	}
	c, err := Handshake(ctx, conn, export, contexts)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

// Handshake runs the fixed newstyle negotiation over conn.
func Handshake(ctx context.Context, conn io.ReadWriteCloser, export string, contexts []string) (*Client, error) {
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	c := &Client{conn: conn, contexts: map[string]uint32{}}
	var hdr struct {
		Magic uint64
		Opt   uint64
		Flags uint16
	}
	if err := binary.Read(conn, binary.BigEndian, &hdr); err != nil {
		return nil, fmt.Errorf("nbd: read greeting: %w", err)
	}
	if hdr.Magic != magicInit || hdr.Opt != magicOpt {
		return nil, errors.New("nbd: not a newstyle NBD server")
	}
	if hdr.Flags&flagFixedNewstyle == 0 {
		return nil, errors.New("nbd: server does not speak fixed newstyle")
	}
	clientFlags := uint32(flagFixedNewstyle)
	if hdr.Flags&flagNoZeroes != 0 {
		clientFlags |= flagNoZeroes
	}
	if err := binary.Write(conn, binary.BigEndian, clientFlags); err != nil {
		return nil, fmt.Errorf("nbd: send flags: %w", err)
	}

	if err := c.sendOpt(optStructuredReply, nil); err != nil {
		return nil, err
	}
	if err := c.expectAck(optStructuredReply); err != nil {
		return nil, fmt.Errorf("nbd: structured replies: %w", err)
	}

	if len(contexts) > 0 {
		payload := exportField(export)
		payload = binary.BigEndian.AppendUint32(payload, uint32(len(contexts))) //nolint:gosec // G115: a handful of names
		for _, name := range contexts {
			payload = binary.BigEndian.AppendUint32(payload, uint32(len(name))) //nolint:gosec // G115: short names
			payload = append(payload, name...)
		}
		if err := c.sendOpt(optSetMetaContext, payload); err != nil {
			return nil, err
		}
		for {
			typ, data, err := c.readOptReply(optSetMetaContext)
			if err != nil {
				return nil, fmt.Errorf("nbd: meta context: %w", err)
			}
			if typ == repAck {
				break
			}
			if typ == repMetaContext && len(data) >= 4 {
				c.contexts[string(data[4:])] = binary.BigEndian.Uint32(data[:4])
			}
		}
	}

	payload := exportField(export)
	payload = binary.BigEndian.AppendUint16(payload, 0)
	if err := c.sendOpt(optGo, payload); err != nil {
		return nil, err
	}
	gotSize := false
	for {
		typ, data, err := c.readOptReply(optGo)
		if err != nil {
			return nil, fmt.Errorf("nbd: open export %q: %w", export, err)
		}
		if typ == repAck {
			break
		}
		if typ == repInfo && len(data) >= 12 && binary.BigEndian.Uint16(data[:2]) == infoExport {
			c.size = int64(binary.BigEndian.Uint64(data[2:10])) //nolint:gosec // G115: a disk size fits
			gotSize = true
		}
	}
	if !gotSize {
		return nil, fmt.Errorf("nbd: export %q reported no size", export)
	}
	return c, nil
}

func exportField(name string) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(len(name))) //nolint:gosec // G115: short name
	return append(b, name...)
}

func (c *Client) sendOpt(opt uint32, data []byte) error {
	buf := binary.BigEndian.AppendUint64(nil, magicOpt)
	buf = binary.BigEndian.AppendUint32(buf, opt)
	buf = binary.BigEndian.AppendUint32(buf, uint32(len(data))) //nolint:gosec // G115: option payloads are small
	buf = append(buf, data...)
	if _, err := c.conn.Write(buf); err != nil {
		return fmt.Errorf("nbd: send option %d: %w", opt, err)
	}
	return nil
}

func (c *Client) readOptReply(opt uint32) (uint32, []byte, error) {
	var r struct {
		Magic  uint64
		Opt    uint32
		Type   uint32
		Length uint32
	}
	if err := binary.Read(c.conn, binary.BigEndian, &r); err != nil {
		return 0, nil, err
	}
	if r.Magic != magicOptReply || r.Opt != opt {
		return 0, nil, errors.New("unexpected option reply")
	}
	if r.Length > 1<<20 {
		return 0, nil, errors.New("option reply too large")
	}
	data := make([]byte, r.Length)
	if _, err := io.ReadFull(c.conn, data); err != nil {
		return 0, nil, err
	}
	if r.Type&repErrBit != 0 {
		if len(data) > 0 {
			return 0, nil, fmt.Errorf("server refused: %s", data)
		}
		return 0, nil, fmt.Errorf("server refused (error %#x)", r.Type)
	}
	return r.Type, data, nil
}

func (c *Client) expectAck(opt uint32) error {
	typ, _, err := c.readOptReply(opt)
	if err == nil && typ != repAck {
		err = fmt.Errorf("unexpected reply type %d", typ)
	}
	return err
}

// Size is the export's size in bytes.
func (c *Client) Size() int64 { return c.size }

// HasContext reports whether the server agreed to the named meta context.
func (c *Client) HasContext(name string) bool {
	_, ok := c.contexts[name]
	return ok
}

// Close ends the session and closes the connection.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.writeRequest(cmdDisc, 0, 0)
	return c.conn.Close()
}

func (c *Client) writeRequest(typ uint16, off, length uint64) error {
	c.cookie++
	buf := make([]byte, 0, 28)
	buf = binary.BigEndian.AppendUint32(buf, magicRequest)
	buf = binary.BigEndian.AppendUint16(buf, 0)
	buf = binary.BigEndian.AppendUint16(buf, typ)
	buf = binary.BigEndian.AppendUint64(buf, c.cookie)
	buf = binary.BigEndian.AppendUint64(buf, off)
	buf = binary.BigEndian.AppendUint32(buf, uint32(length)) //nolint:gosec // G115: callers cap length
	_, err := c.conn.Write(buf)
	return err
}

// chunk is one structured reply chunk, or a simple reply turned into one.
type chunk struct {
	flags uint16
	typ   uint16
	data  []byte
}

func (c *Client) readChunk() (chunk, error) {
	var magic uint32
	if err := binary.Read(c.conn, binary.BigEndian, &magic); err != nil {
		return chunk{}, err
	}
	switch magic {
	case magicSimple:
		var r struct {
			Error  uint32
			Cookie uint64
		}
		if err := binary.Read(c.conn, binary.BigEndian, &r); err != nil {
			return chunk{}, err
		}
		if r.Error != 0 {
			return chunk{}, fmt.Errorf("nbd: request failed (error %d)", r.Error)
		}
		return chunk{flags: replyFlagDone, typ: replyNone}, nil
	case magicStructure:
		var r struct {
			Flags  uint16
			Type   uint16
			Cookie uint64
			Length uint32
		}
		if err := binary.Read(c.conn, binary.BigEndian, &r); err != nil {
			return chunk{}, err
		}
		if r.Cookie != c.cookie {
			return chunk{}, errors.New("nbd: reply for another request")
		}
		if r.Length > maxRead+64 {
			return chunk{}, errors.New("nbd: reply chunk too large")
		}
		data := make([]byte, r.Length)
		if _, err := io.ReadFull(c.conn, data); err != nil {
			return chunk{}, err
		}
		if r.Type&replyErrBit != 0 {
			msg := ""
			if len(data) >= 6 {
				n := int(binary.BigEndian.Uint16(data[4:6]))
				if 6+n <= len(data) {
					msg = string(data[6 : 6+n])
				}
			}
			return chunk{}, fmt.Errorf("nbd: request failed: %s", msg)
		}
		return chunk{flags: r.Flags, typ: r.Type, data: data}, nil
	default:
		return chunk{}, errors.New("nbd: bad reply magic")
	}
}

// ReadAt fills p from the export at off. Ranges the server reports as holes
// read as zeros.
func (c *Client) ReadAt(p []byte, off int64) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if off < 0 || off+int64(len(p)) > c.size {
		return 0, fmt.Errorf("nbd: read of %d bytes at %d is outside the export", len(p), off)
	}
	done := 0
	for done < len(p) {
		n := min(len(p)-done, maxRead)
		if err := c.readOne(p[done:done+n], off+int64(done)); err != nil {
			return done, err
		}
		done += n
	}
	return done, nil
}

func (c *Client) readOne(p []byte, off int64) error {
	if err := c.writeRequest(cmdRead, uint64(off), uint64(len(p))); err != nil { //nolint:gosec // G115: off is checked non-negative
		return fmt.Errorf("nbd: send read: %w", err)
	}
	clear(p)
	covered := 0
	for {
		ch, err := c.readChunk()
		if err != nil {
			return err
		}
		switch ch.typ {
		case replyOffsetData:
			if len(ch.data) < 8 {
				return errors.New("nbd: short data chunk")
			}
			at := int64(binary.BigEndian.Uint64(ch.data[:8])) - off //nolint:gosec // G115: offsets within the export
			body := ch.data[8:]
			if at < 0 || at+int64(len(body)) > int64(len(p)) {
				return errors.New("nbd: data chunk outside the request")
			}
			copy(p[at:], body)
			covered += len(body)
		case replyOffsetHole:
			if len(ch.data) < 12 {
				return errors.New("nbd: short hole chunk")
			}
			at := int64(binary.BigEndian.Uint64(ch.data[:8])) - off //nolint:gosec // G115: offsets within the export
			n := int64(binary.BigEndian.Uint32(ch.data[8:12]))
			if at < 0 || at+n > int64(len(p)) {
				return errors.New("nbd: hole chunk outside the request")
			}
			covered += int(n)
		case replyNone:
		default:
			return fmt.Errorf("nbd: unexpected reply type %d to a read", ch.typ)
		}
		if ch.flags&replyFlagDone != 0 {
			break
		}
	}
	if covered != len(p) {
		return fmt.Errorf("nbd: read at %d returned %d of %d bytes", off, covered, len(p))
	}
	return nil
}

// BlockStatus returns the extents the named context reports for
// [off, off+length), in order and covering the whole range.
func (c *Client) BlockStatus(name string, off, length int64) ([]Extent, error) {
	id, ok := c.contexts[name]
	if !ok {
		return nil, fmt.Errorf("nbd: meta context %q was not negotiated", name)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	end := min(off+length, c.size)
	var out []Extent
	for pos := off; pos < end; {
		ask := min(end-pos, maxStatus)
		exts, err := c.statusOne(id, pos, ask)
		if err != nil {
			return nil, err
		}
		if len(exts) == 0 {
			return nil, fmt.Errorf("nbd: no block status at %d", pos)
		}
		for _, e := range exts {
			if pos >= end {
				break
			}
			e.Length = min(e.Length, end-pos)
			e.Offset = pos
			out = append(out, e)
			pos += e.Length
		}
	}
	return out, nil
}

func (c *Client) statusOne(id uint32, off, length int64) ([]Extent, error) {
	if err := c.writeRequest(cmdBlockStatus, uint64(off), uint64(length)); err != nil { //nolint:gosec // G115: off is non-negative
		return nil, fmt.Errorf("nbd: send block status: %w", err)
	}
	var exts []Extent
	for {
		ch, err := c.readChunk()
		if err != nil {
			return nil, err
		}
		if ch.typ == replyBlockStatus && len(ch.data) >= 4 && binary.BigEndian.Uint32(ch.data[:4]) == id {
			for d := ch.data[4:]; len(d) >= 8; d = d[8:] {
				n := int64(binary.BigEndian.Uint32(d[:4]))
				if n == 0 {
					return nil, errors.New("nbd: empty extent")
				}
				exts = append(exts, Extent{Length: n, Flags: binary.BigEndian.Uint32(d[4:8])})
			}
		}
		if ch.flags&replyFlagDone != 0 {
			return exts, nil
		}
	}
}
