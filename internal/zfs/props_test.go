package zfs

import (
	"reflect"
	"testing"
)

// Output of zfs get -H -p -r -t filesystem -s local,none on a zfs 2.4 host,
// shortened to the lines that matter.
const propsOutput = "tank/data\ttype\tfilesystem\t-\n" +
	"tank/data\tused\t49152\t-\n" +
	"tank/data\trecordsize\t1048576\tlocal\n" +
	"tank/data\tcompression\tzstd\tlocal\n" +
	"tank/data\tutf8only\ton\t-\n" +
	"tank/data\tnormalization\tformD\t-\n" +
	"tank/data\tcasesensitivity\tinsensitive\t-\n" +
	"tank/data\tcom.example:tag\tx\tlocal\n" +
	"tank/data/child\tquota\t104857600\tlocal\n" +
	"tank/data/child\tutf8only\toff\t-\n" +
	"tank/data/child\tnormalization\tnone\t-\n" +
	"tank/data/child\tcasesensitivity\tsensitive\t-\n" +
	"tank/data/child\tmountpoint\t/mnt/elsewhere\tlocal\n"

func TestParsePropertiesKeepsLocalAndChangedCreationTimeOnes(t *testing.T) {
	got, err := ParseProperties(propsOutput)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Properties{
		"tank/data": {
			"recordsize": "1048576", "compression": "zstd", "com.example:tag": "x",
			"utf8only": "on", "normalization": "formD", "casesensitivity": "insensitive",
		},
		"tank/data/child": {"quota": "104857600", "mountpoint": "/mnt/elsewhere"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestParsePropertiesRefusesAShortLine(t *testing.T) {
	if _, err := ParseProperties("tank\tcompression\tlz4\n"); err == nil {
		t.Fatal("a line without a source was accepted")
	}
}

func TestCreatePassesCreationTimePropertiesButNotTheMountpoint(t *testing.T) {
	args, err := CreateArgs("tank/restored", Properties{
		"casesensitivity": "insensitive", "compression": "zstd",
		"mountpoint": "/mnt/tank/data", "readonly": "on", "keylocation": "file:///k",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "create", "-o", "casesensitivity=insensitive", "-o", "compression=zstd", "tank/restored"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
}

func TestSetLeavesOutWhatOnlyCreateCanTake(t *testing.T) {
	args, err := SetArgs("tank/data", Properties{"casesensitivity": "insensitive", "atime": "off", "quota": "1024", "canmount": "off"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"zfs", "set", "atime=off", "quota=1024", "tank/data"}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	if args, err := SetArgs("tank/data", Properties{"utf8only": "on"}); err != nil || args != nil {
		t.Fatalf("nothing settable must give no command, got %v %v", args, err)
	}
}

func TestPropertyArgsRefuseWhatZfsGetCannotHaveSaid(t *testing.T) {
	for _, p := range []Properties{{"compression": "lz4\nzfs destroy"}, {"-o x": "y"}, {"a=b": "c"}} {
		if _, err := CreateArgs("tank/x", p); err == nil {
			t.Errorf("%v was accepted", p)
		}
	}
	if _, err := CreateArgs("tank/../x", nil); err == nil {
		t.Error("an invalid dataset name was accepted")
	}
}
