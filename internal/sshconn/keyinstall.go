package sshconn

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// The ways InstallKey fails that a person can do something about.
var (
	ErrPasswordRefused  = errors.New("the server refused the password")
	ErrNoPasswordSignIn = errors.New("the server does not take a password for this account")
	ErrSessionRefused   = errors.New("the server refused a session for this account")
	ErrUnreachable      = errors.New("the server cannot be reached")
	ErrHostKeyChanged   = errors.New("the server's host key differs from the one pinned for it")
	ErrNoWayIn          = errors.New("the server offers neither SFTP nor a shell to add the key with")
)

// KeyTarget is the account on a server that InstallKey puts a key on.
type KeyTarget struct {
	Host, User, Port string
	// StorageBox selects Hetzner's install-ssh-key command, which a Storage
	// Box offers on port 23 in place of a shell.
	StorageBox bool
	// KnownHosts is the file the server's host key is pinned in on first
	// contact and checked against after that, as ssh's accept-new does.
	KnownHosts string
}

// InstallKey signs in to the target once with password and adds pubKey to the
// account's authorized keys. The password goes into this one handshake only.
// The deadline of ctx bounds the whole exchange.
func InstallKey(ctx context.Context, t KeyTarget, password, pubKey string) error {
	want, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKey))
	if err != nil {
		return fmt.Errorf("sshconn: the public key does not parse: %w", err)
	}
	client, err := dialPassword(ctx, t, password)
	if err != nil {
		return err
	}
	defer client.Close() //nolint:errcheck // the key is in place or the error says why
	stop := context.AfterFunc(ctx, func() { _ = client.Close() })
	defer stop()
	if t.StorageBox {
		return storageBoxInstall(client, pubKey)
	}
	err = appendOverSFTP(client, want, pubKey)
	if errors.Is(err, errNoSFTP) {
		err = appendOverShell(client, want, pubKey)
	}
	return err
}

func dialPassword(ctx context.Context, t KeyTarget, password string) (*ssh.Client, error) {
	hostKeys, algorithms, err := pinOnFirstUse(t.KnownHosts, t.Host, t.Port)
	if err != nil {
		return nil, err
	}
	addr := net.JoinHostPort(t.Host, t.Port)
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	cfg := &ssh.ClientConfig{
		User:              t.User,
		Auth:              []ssh.AuthMethod{ssh.Password(password), ssh.KeyboardInteractive(passwordPrompt(password))},
		HostKeyCallback:   hostKeys,
		HostKeyAlgorithms: algorithms,
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, handshakeError(err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// passwordPrompt answers a keyboard-interactive round that asks for the
// password alone, which is how a PAM server without password authentication
// takes one. A round that asks for more, such as a second factor, is refused.
func passwordPrompt(password string) ssh.KeyboardInteractiveChallenge {
	return func(_, _ string, questions []string, echos []bool) ([]string, error) {
		switch {
		case len(questions) == 0:
			return nil, nil
		case len(questions) == 1 && !echos[0]:
			return []string{password}, nil
		}
		return nil, errors.New("the server asks for more than a password")
	}
}

func handshakeError(err error) error {
	msg := err.Error()
	var netErr net.Error
	switch {
	case errors.Is(err, ErrHostKeyChanged):
		return ErrHostKeyChanged
	case strings.Contains(msg, "unable to authenticate"):
		if strings.Contains(msg, "password") || strings.Contains(msg, "keyboard-interactive") {
			return ErrPasswordRefused
		}
		return ErrNoPasswordSignIn
	case errors.As(err, &netErr) && netErr.Timeout():
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	case errors.Is(err, io.EOF), strings.Contains(msg, "connection reset"):
		return ErrSessionRefused
	}
	return fmt.Errorf("sshconn: %w", err)
}

// pinOnFirstUse checks host keys against the file at path and adds the key of
// a host it has none for. It also returns the algorithms of the keys pinned
// for the host, so that a server with several host keys shows the one on file
// instead of one that would look like a changed key.
func pinOnFirstUse(path, host, port string) (ssh.HostKeyCallback, []string, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, nil, fmt.Errorf("sshconn: mkdir: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600) //nolint:gosec // G304: the path is BombVault's own known_hosts
	if err != nil {
		return nil, nil, fmt.Errorf("sshconn: known_hosts: %w", err)
	}
	_ = f.Close()
	known, err := knownhosts.New(path)
	if err != nil {
		return nil, nil, fmt.Errorf("sshconn: known_hosts: %w", err)
	}
	algorithms, err := pinnedAlgorithms(known, net.JoinHostPort(host, port))
	if err != nil {
		return nil, nil, err
	}
	callback := func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := known(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		switch {
		case err == nil:
			return nil
		case errors.As(err, &keyErr) && len(keyErr.Want) == 0:
			return appendKnownHost(path, hostname, key)
		case errors.As(err, &keyErr):
			return ErrHostKeyChanged
		}
		return err
	}
	return callback, algorithms, nil
}

// pinnedAlgorithms asks known with a key no host has, which makes it name the
// keys it holds for addr.
func pinnedAlgorithms(known ssh.HostKeyCallback, addr string) ([]string, error) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	probe, err := ssh.NewPublicKey(pub)
	if err != nil {
		return nil, err
	}
	var keyErr *knownhosts.KeyError
	if !errors.As(known(addr, &net.TCPAddr{}, probe), &keyErr) {
		return nil, nil
	}
	var algorithms []string
	for _, k := range keyErr.Want {
		if k.Key.Type() == ssh.KeyAlgoRSA {
			algorithms = append(algorithms, ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256)
		}
		algorithms = append(algorithms, k.Key.Type())
	}
	return algorithms, nil
}

func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // G304: the path is BombVault's own known_hosts
	if err != nil {
		return fmt.Errorf("sshconn: known_hosts: %w", err)
	}
	line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key) + "\n"
	if _, err := f.WriteString(line); err != nil {
		_ = f.Close()
		return fmt.Errorf("sshconn: known_hosts: %w", err)
	}
	return f.Close()
}

// storageBoxInstall runs the command a Storage Box keeps for this on port 23.
// It works for the main account and for a sub-account with SSH support on.
func storageBoxInstall(client *ssh.Client, pubKey string) error {
	s, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSessionRefused, err)
	}
	defer s.Close() //nolint:errcheck // the command's exit status is what counts
	s.Stdin = strings.NewReader(pubKey + "\n")
	out, err := s.CombinedOutput("install-ssh-key")
	var exitErr *ssh.ExitError
	var missing *ssh.ExitMissingError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &exitErr):
		return fmt.Errorf("sshconn: install-ssh-key: %s", lastLine(string(out)))
	case errors.As(err, &missing):
		return ErrSessionRefused
	}
	return fmt.Errorf("%w: %w", ErrSessionRefused, err)
}

// errNoSFTP is what appendOverSFTP returns when the server has no SFTP
// subsystem, which leaves the shell to try.
var errNoSFTP = errors.New("no sftp subsystem")

// authorizedKeysLimit caps how much of authorized_keys is read to look for
// the key; a real file is a few kilobytes.
const authorizedKeysLimit = 1 << 20

func appendOverSFTP(client *ssh.Client, want ssh.PublicKey, pubKey string) error {
	s, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSessionRefused, err)
	}
	defer s.Close() //nolint:errcheck // the sftp client below reports what failed
	stdin, err := s.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		return err
	}
	if err := s.RequestSubsystem("sftp"); err != nil {
		return errNoSFTP
	}
	c, err := sftp.NewClientPipe(stdout, stdin)
	if err != nil {
		return errNoSFTP
	}
	defer c.Close() //nolint:errcheck // the write below is checked on its own

	// Paths without a leading slash are taken from the login folder, which is
	// home for a normal account and the root of a chroot for an SFTP-only one.
	if _, err := c.Stat(".ssh"); errors.Is(err, os.ErrNotExist) {
		if err := c.Mkdir(".ssh"); err != nil {
			return fmt.Errorf("sshconn: create ~/.ssh: %w", err)
		}
		if err := c.Chmod(".ssh", 0o700); err != nil {
			return fmt.Errorf("sshconn: chmod ~/.ssh: %w", err)
		}
	}
	const file = ".ssh/authorized_keys"
	_, statErr := c.Stat(file)
	f, err := c.OpenFile(file, os.O_RDWR|os.O_CREATE)
	if err != nil {
		return fmt.Errorf("sshconn: open ~/%s: %w", file, err)
	}
	defer f.Close() //nolint:errcheck // a failed write is reported by Write
	if errors.Is(statErr, os.ErrNotExist) {
		if err := c.Chmod(file, 0o600); err != nil {
			return fmt.Errorf("sshconn: chmod ~/%s: %w", file, err)
		}
	}
	have, err := io.ReadAll(io.LimitReader(f, authorizedKeysLimit))
	if err != nil {
		return fmt.Errorf("sshconn: read ~/%s: %w", file, err)
	}
	if hasKey(have, want) {
		return nil
	}
	line := pubKey + "\n"
	if len(have) > 0 && !bytes.HasSuffix(have, []byte("\n")) {
		line = "\n" + line
	}
	if _, err := f.Write([]byte(line)); err != nil {
		return fmt.Errorf("sshconn: write ~/%s: %w", file, err)
	}
	return nil
}

// appendOverShell adds the key with a POSIX shell on a server that runs
// commands but has no SFTP. The umask makes a new ~/.ssh 700 and a new
// authorized_keys 600 and leaves existing ones as they are.
func appendOverShell(client *ssh.Client, want ssh.PublicKey, pubKey string) error {
	s, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrSessionRefused, err)
	}
	defer s.Close() //nolint:errcheck // the command's exit status is what counts
	out, err := s.CombinedOutput(shellAppendScript(want, pubKey))
	var exitErr *ssh.ExitError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &exitErr) && exitErr.ExitStatus() != 126 && exitErr.ExitStatus() != 127:
		return fmt.Errorf("sshconn: adding the key failed: %s", lastLine(string(out)))
	}
	return ErrNoWayIn
}

func shellAppendScript(want ssh.PublicKey, pubKey string) string {
	body := shellQuote(strings.TrimSpace(string(ssh.MarshalAuthorizedKey(want))))
	f := "~/.ssh/authorized_keys"
	return "umask 077 && mkdir -p ~/.ssh && touch " + f + " && { grep -qF " + body + " " + f +
		` || { if [ -s ` + f + ` ] && [ -n "$(tail -c 1 ` + f + `)" ]; then echo; fi; echo ` + shellQuote(pubKey) + "; } >> " + f + "; }"
}

// hasKey reports whether the authorized_keys content in have lists want,
// with any options or comment.
func hasKey(have []byte, want ssh.PublicKey) bool {
	wantBytes := want.Marshal()
	for rest := have; len(rest) > 0; {
		key, _, _, next, err := ssh.ParseAuthorizedKey(rest)
		if err != nil {
			return false
		}
		if bytes.Equal(key.Marshal(), wantBytes) {
			return true
		}
		rest = next
	}
	return false
}
