package restic

import (
	"slices"
	"testing"
	"time"
)

func TestBackupDirFromArgsPutsParentAndIgnoresBeforeTheSeparator(t *testing.T) {
	args := BackupDirFromArgs("/repo", "abc123", []string{"vm:x"}, Mode{Encrypted: true})
	sep := slices.Index(args, "--")
	if sep < 0 || args[len(args)-1] != "." || sep != len(args)-2 {
		t.Fatalf("positional not after --: %v", args)
	}
	for _, flag := range []string{"--ignore-inode", "--ignore-ctime", "--parent"} {
		i := slices.Index(args, flag)
		if i < 0 || i > sep {
			t.Fatalf("%s missing or after --: %v", flag, args)
		}
	}
	if args[slices.Index(args, "--parent")+1] != "abc123" {
		t.Fatalf("parent id not passed: %v", args)
	}
	full := BackupDirFromArgs("/repo", "", nil, Mode{Encrypted: true})
	if slices.Contains(full, "--parent") || !slices.Contains(full, "--force") {
		t.Fatalf("a run without parent must read every file: %v", full)
	}
}

func TestParseNodeTimesKeepsNodesWithMtime(t *testing.T) {
	out := []byte(`{"time":"2026-09-27T20:16:47Z","tree":"2e","paths":["/w/st"],"struct_type":"snapshot","message_type":"snapshot"}
{"name":"vda","type":"dir","path":"/vda","mtime":"2026-09-27T20:16:47.316002269Z","message_type":"node","struct_type":"node"}
{"name":"00000000","type":"file","path":"/vda/00000000","size":1048576,"mtime":"1970-01-01T02:46:40Z","message_type":"node","struct_type":"node"}
`)
	nodes, err := parseNodeTimes(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("nodes = %+v", nodes)
	}
	f := nodes[1]
	if f.Path != "/vda/00000000" || f.Type != "file" || f.Size != 1048576 || !f.Mtime.Equal(time.Unix(10000, 0)) {
		t.Fatalf("file node = %+v", f)
	}
	if _, err := parseNodeTimes([]byte(`{"struct_type":"snapshot"}`)); err == nil {
		t.Fatal("a listing without nodes is not an error")
	}
}
