package remotes

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"

	"golang.org/x/crypto/ssh"
)

// HostKey is the key an SSH server proves itself with.
type HostKey struct {
	// Pin is the key as rclone's sftp option host_keys takes it.
	Pin string `json:"pin"`
	// Fingerprint is the SHA256 fingerprint providers publish and ssh prints.
	Fingerprint string `json:"fingerprint"`
}

var errGotHostKey = errors.New("host key read")

// ReadHostKey asks an SSH server for its host key and hangs up before
// signing in. rclone negotiates only the algorithms of the keys pinned in
// host_keys, so the one key read here is all a remote needs. Certificates are
// not asked for, since rclone cannot pin one.
func ReadHostKey(ctx context.Context, host, port string) (HostKey, error) {
	if port == "" {
		port = "22"
	}
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	addr := net.JoinHostPort(host, port)
	conn, err := new(net.Dialer).DialContext(ctx, "tcp", addr)
	if err != nil {
		return HostKey{}, fmt.Errorf("read the host key of %s: %w", addr, err)
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	var got ssh.PublicKey
	_, _, _, err = ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User: "bombvault",
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
			ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256,
		},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			got = key
			return errGotHostKey
		},
	})
	if got == nil {
		return HostKey{}, fmt.Errorf("read the host key of %s: %w", addr, err)
	}
	return HostKey{
		Pin:         got.Type() + " " + base64.StdEncoding.EncodeToString(got.Marshal()),
		Fingerprint: ssh.FingerprintSHA256(got),
	}, nil
}
