package api

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"slices"
	"testing"
)

func TestParseSFTPLocation(t *testing.T) {
	for _, tc := range []struct {
		in                     string
		user, host, port, path string
	}{
		{"sftp:backup@nas.lan:/srv/restic", "backup", "nas.lan", "", "/srv/restic"},
		{"sftp:nas.lan:restic/containers", "", "nas.lan", "", "restic/containers"},
		{"sftp:u1@[fd00::2]:/data", "u1", "fd00::2", "", "/data"},
		{"sftp://backup@nas.lan:2222//srv/restic", "backup", "nas.lan", "2222", "/srv/restic"},
		{"sftp://nas.lan/restic", "", "nas.lan", "", "restic"},
		{"sftp:u123@u123.your-storagebox.de:", "u123", "u123.your-storagebox.de", "", ""},
	} {
		loc, err := parseSFTPLocation(tc.in)
		if err != nil {
			t.Errorf("%s: %v", tc.in, err)
			continue
		}
		if loc.user != tc.user || loc.host != tc.host || loc.port != tc.port || loc.path != tc.path {
			t.Errorf("%s: got %+v", tc.in, loc)
		}
	}
	for _, bad := range []string{"sftp:", "sftp:/only/a/path", "s3:bucket", "sftp://", "sftp://nas.lan", "sftp:-oProxyCommand=x:/p"} {
		if _, err := parseSFTPLocation(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}

// The ssh call is the one restic makes for the same location, plus the options
// that keep a probe from ever waiting for a password prompt.
func TestSFTPProbeArgsMirrorRestic(t *testing.T) {
	got := sftpSSHArgs(sftpLocation{user: "backup", host: "nas.lan", port: "2222"})
	want := []string{"-o", "BatchMode=yes", "-o", "ConnectTimeout=10", "nas.lan", "-p", "2222", "-l", "backup", "-s", "sftp"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// fakeSFTPServer answers the client's INIT and one extended request, the way
// OpenSSH's sftp-server does.
type fakeSFTPServer struct {
	extensions []string
	statusOn   string // a path the server answers with SSH_FX_NO_SUCH_FILE
	asked      []string
}

func (f *fakeSFTPServer) serve(in io.Reader, out io.Writer) error {
	for {
		pkt, err := readSFTPPacket(in)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		var reply bytes.Buffer
		switch pkt[0] {
		case sshFxpInit:
			reply.WriteByte(sshFxpVersion)
			_ = binary.Write(&reply, binary.BigEndian, uint32(3))
			for _, ext := range f.extensions {
				writeSFTPString(&reply, ext)
				writeSFTPString(&reply, "2")
			}
		case sshFxpExtended:
			id := pkt[1:5]
			body := pkt[5:]
			name, body := splitSFTPString(body)
			path, _ := splitSFTPString(body)
			f.asked = append(f.asked, name+" "+path)
			if path == f.statusOn {
				reply.WriteByte(sshFxpStatus)
				reply.Write(id)
				_ = binary.Write(&reply, binary.BigEndian, uint32(2))
				writeSFTPString(&reply, "No such file")
				writeSFTPString(&reply, "")
				break
			}
			reply.WriteByte(sshFxpExtendedReply)
			reply.Write(id)
			// bsize, frsize, blocks, bfree, bavail, files, ffree, favail, fsid, flag, namemax
			for _, v := range []uint64{4096, 1024, 1000, 400, 300, 0, 0, 0, 0, 0, 255} {
				_ = binary.Write(&reply, binary.BigEndian, v)
			}
		}
		if err := writeSFTPPacket(out, reply.Bytes()); err != nil {
			return err
		}
	}
}

func splitSFTPString(b []byte) (string, []byte) {
	n := binary.BigEndian.Uint32(b)
	return string(b[4 : 4+n]), b[4+n:]
}

func runFakeSFTP(t *testing.T, srv *fakeSFTPServer, path string) (sftpSpace, error) {
	t.Helper()
	clientR, serverW := io.Pipe()
	serverR, clientW := io.Pipe()
	done := make(chan error, 1)
	go func() {
		done <- srv.serve(serverR, serverW)
		_ = serverW.Close()
	}()
	space, err := sftpStatvfs(clientR, clientW, path)
	_ = clientW.Close()
	if sErr := <-done; sErr != nil {
		t.Fatalf("fake server: %v", sErr)
	}
	return space, err
}

func TestSFTPStatvfsReadsTheFragmentSize(t *testing.T) {
	srv := &fakeSFTPServer{extensions: []string{"posix-rename@openssh.com", "statvfs@openssh.com"}}
	space, err := runFakeSFTP(t, srv, "/srv/restic")
	if err != nil {
		t.Fatal(err)
	}
	// statvfs counts blocks in fragment units, as df does.
	if space.free != 300*1024 || space.total != 1000*1024 {
		t.Fatalf("free %d total %d, want %d and %d", space.free, space.total, 300*1024, 1000*1024)
	}
	if !slices.Equal(srv.asked, []string{"statvfs@openssh.com /srv/restic"}) {
		t.Fatalf("asked %v", srv.asked)
	}
}

// A repository that is not there yet sits on the same filesystem as the
// account's home, which is the next best answer.
func TestSFTPStatvfsFallsBackToTheHomeDirectory(t *testing.T) {
	srv := &fakeSFTPServer{extensions: []string{"statvfs@openssh.com"}, statusOn: "restic/new"}
	space, err := runFakeSFTP(t, srv, "restic/new")
	if err != nil {
		t.Fatal(err)
	}
	if space.free == 0 || !slices.Equal(srv.asked, []string{"statvfs@openssh.com restic/new", "statvfs@openssh.com ."}) {
		t.Fatalf("free %d, asked %v", space.free, srv.asked)
	}
}

func TestSFTPStatvfsWithoutTheExtensionIsUnsupported(t *testing.T) {
	srv := &fakeSFTPServer{extensions: []string{"posix-rename@openssh.com"}}
	if _, err := runFakeSFTP(t, srv, "/srv"); !errors.Is(err, errAboutUnsupported) {
		t.Fatalf("err = %v, want errAboutUnsupported", err)
	}
	if len(srv.asked) != 0 {
		t.Fatalf("a server that does not offer statvfs must not be asked, got %v", srv.asked)
	}
}
