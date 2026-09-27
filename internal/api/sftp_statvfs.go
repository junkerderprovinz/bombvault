package api

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path"
	"strings"
	"time"
)

// sftpProbeTimeout bounds the whole probe, handshake included, like
// rcloneAboutTimeout does for rclone.
const sftpProbeTimeout = 30 * time.Second

// SFTP version 3 packet types (draft-ietf-secsh-filexfer-02) and the OpenSSH
// extension that reports a filesystem's size.
const (
	sshFxpInit          = 1
	sshFxpVersion       = 2
	sshFxpStatus        = 101
	sshFxpExtended      = 200
	sshFxpExtendedReply = 201

	statvfsExtension = "statvfs@openssh.com"
	// sftpMaxPacket caps what the probe reads. A statvfs reply is 93 bytes and
	// a version reply lists a handful of extensions.
	sftpMaxPacket = 256 * 1024
)

// sftpLocation is what restic's sftp backend reads out of a repository
// location, and so what it hands to ssh.
type sftpLocation struct {
	user, host, port, path string
}

// parseSFTPLocation reads both forms restic accepts, sftp:user@host:path and
// sftp://user@host:port//path, the same way restic does.
func parseSFTPLocation(loc string) (sftpLocation, error) {
	var out sftpLocation
	switch {
	case strings.HasPrefix(loc, "sftp://"):
		u, err := url.Parse(loc)
		if err != nil {
			return out, fmt.Errorf("not an sftp location: %w", err)
		}
		if u.User != nil {
			out.user = u.User.Username()
		}
		out.host, out.port = u.Hostname(), u.Port()
		if u.Path == "" {
			return out, errors.New("this sftp location names no directory")
		}
		out.path = u.Path[1:]
	case strings.HasPrefix(loc, "sftp:"):
		rest := loc[len("sftp:"):]
		userHost, dir, ok := cutSFTPHost(rest)
		if !ok {
			return out, errors.New("this sftp location names no directory")
		}
		if i := strings.LastIndex(userHost, "@"); i >= 0 {
			out.user, userHost = userHost[:i], userHost[i+1:]
		}
		out.host = strings.TrimSuffix(strings.TrimPrefix(userHost, "["), "]")
		out.path = dir
	default:
		return out, errors.New("not an sftp location")
	}
	if out.host == "" {
		return out, errors.New("this sftp location names no host")
	}
	// ssh would read such a host as an option.
	if strings.HasPrefix(out.host, "-") {
		return out, errors.New("an sftp host must not start with a dash")
	}
	if out.path != "" {
		out.path = path.Clean(out.path)
	}
	return out, nil
}

// cutSFTPHost splits "user@host:path" at the colon after the host, stepping
// over a bracketed IPv6 address.
func cutSFTPHost(s string) (host, dir string, ok bool) {
	if at := strings.LastIndex(s, "@["); at >= 0 || strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 || end+1 >= len(s) || s[end+1] != ':' {
			return "", "", false
		}
		return s[:end+1], s[end+2:], true
	}
	return strings.Cut(s, ":")
}

// sftpSSHArgs is the argv restic's sftp backend gives ssh for loc, behind two
// options that make a probe fail instead of waiting for a password or a slow
// host.
func sftpSSHArgs(loc sftpLocation) []string {
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", loc.host}
	if loc.port != "" {
		args = append(args, "-p", loc.port)
	}
	if loc.user != "" {
		args = append(args, "-l", loc.user)
	}
	return append(args, "-s", "sftp")
}

// sftpSpace is a filesystem's room as statvfs reports it.
type sftpSpace struct {
	free, total uint64
}

// sftpCapacity asks the SFTP server behind a repository location how much
// room its filesystem has, over the same ssh connection restic opens.
func sftpCapacity(ctx context.Context, repo string) (aboutResult, error) {
	loc, err := parseSFTPLocation(repo)
	if err != nil {
		return aboutResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, sftpProbeTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", sftpSSHArgs(loc)...) //nolint:gosec // G204: host, user and port come from a stored repository location and cannot start an option
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return aboutResult{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return aboutResult{}, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return aboutResult{}, fmt.Errorf("ssh: %w", err)
	}
	space, pErr := sftpStatvfs(stdout, stdin, loc.path)
	_ = stdin.Close()
	wErr := cmd.Wait()
	if pErr != nil {
		if errors.Is(pErr, errAboutUnsupported) {
			return aboutResult{}, pErr
		}
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return aboutResult{}, fmt.Errorf("sftp: %w (%s)", pErr, msg)
		}
		return aboutResult{}, fmt.Errorf("sftp: %w", pErr)
	}
	if wErr != nil && ctx.Err() != nil {
		return aboutResult{}, ctx.Err()
	}
	total := clampToInt64(space.total)
	return aboutResult{Free: clampToInt64(space.free), Total: &total}, nil
}

// sftpStatvfs speaks just enough SFTP to ask one question: it sends INIT,
// checks that the server offers statvfs@openssh.com, and asks it about dir.
// A directory the server cannot stat, usually a repository not created yet,
// is asked again as the login directory, which sits on the same filesystem in
// every setup this is for.
func sftpStatvfs(r io.Reader, w io.Writer, dir string) (sftpSpace, error) {
	var init bytes.Buffer
	init.WriteByte(sshFxpInit)
	_ = binary.Write(&init, binary.BigEndian, uint32(3))
	if err := writeSFTPPacket(w, init.Bytes()); err != nil {
		return sftpSpace{}, err
	}
	version, err := readSFTPPacket(r)
	if err != nil {
		return sftpSpace{}, err
	}
	if version[0] != sshFxpVersion || len(version) < 5 {
		return sftpSpace{}, errors.New("the server did not answer with an SFTP version")
	}
	if !sftpOffers(version[5:], statvfsExtension) {
		return sftpSpace{}, errAboutUnsupported
	}

	paths := []string{"."}
	if dir != "" && dir != "." {
		paths = []string{dir, "."}
	}
	var lastErr error
	for i, p := range paths {
		space, err := sftpAskStatvfs(r, w, uint32(i+1), p)
		if err == nil {
			return space, nil
		}
		lastErr = err
	}
	return sftpSpace{}, lastErr
}

func sftpAskStatvfs(r io.Reader, w io.Writer, id uint32, dir string) (sftpSpace, error) {
	var req bytes.Buffer
	req.WriteByte(sshFxpExtended)
	_ = binary.Write(&req, binary.BigEndian, id)
	writeSFTPString(&req, statvfsExtension)
	writeSFTPString(&req, dir)
	if err := writeSFTPPacket(w, req.Bytes()); err != nil {
		return sftpSpace{}, err
	}
	reply, err := readSFTPPacket(r)
	if err != nil {
		return sftpSpace{}, err
	}
	if len(reply) < 5 || binary.BigEndian.Uint32(reply[1:5]) != id {
		return sftpSpace{}, errors.New("the server answered a question the probe did not ask")
	}
	switch reply[0] {
	case sshFxpStatus:
		return sftpSpace{}, fmt.Errorf("the server could not measure %q", dir)
	case sshFxpExtendedReply:
	default:
		return sftpSpace{}, fmt.Errorf("unexpected SFTP reply type %d", reply[0])
	}
	body := reply[5:]
	if len(body) < 5*8 {
		return sftpSpace{}, errors.New("the statvfs reply is too short")
	}
	field := func(i int) uint64 { return binary.BigEndian.Uint64(body[i*8:]) }
	frsize := field(1)
	if frsize == 0 {
		frsize = field(0)
	}
	return sftpSpace{free: field(4) * frsize, total: field(2) * frsize}, nil
}

// sftpOffers reports whether a version reply's extension list names ext.
func sftpOffers(exts []byte, ext string) bool {
	for len(exts) >= 4 {
		name, rest, ok := readSFTPString(exts)
		if !ok {
			return false
		}
		_, rest, ok = readSFTPString(rest)
		if !ok {
			return false
		}
		if name == ext {
			return true
		}
		exts = rest
	}
	return false
}

func readSFTPString(b []byte) (string, []byte, bool) {
	if len(b) < 4 {
		return "", nil, false
	}
	n := binary.BigEndian.Uint32(b)
	if int64(n) > int64(len(b)-4) {
		return "", nil, false
	}
	return string(b[4 : 4+n]), b[4+n:], true
}

func writeSFTPString(buf *bytes.Buffer, s string) {
	_ = binary.Write(buf, binary.BigEndian, uint32(len(s))) //nolint:gosec // G115: the strings are a path and an extension name
	buf.WriteString(s)
}

func writeSFTPPacket(w io.Writer, payload []byte) error {
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(payload))) //nolint:gosec // G115: the probe's packets are a few hundred bytes
	if _, err := w.Write(append(hdr[:], payload...)); err != nil {
		return fmt.Errorf("sftp: write: %w", err)
	}
	return nil
}

func readSFTPPacket(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 || n > sftpMaxPacket {
		return nil, fmt.Errorf("sftp: a %d-byte packet is not an answer to this probe", n)
	}
	pkt := make([]byte, n)
	if _, err := io.ReadFull(r, pkt); err != nil {
		return nil, err
	}
	return pkt, nil
}
