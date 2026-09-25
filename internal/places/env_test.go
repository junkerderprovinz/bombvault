package places

import (
	"slices"
	"testing"
)

func TestRemoteNameKeepsToWhatAnEnvironmentVariableAllows(t *testing.T) {
	if got := RemoteName("0A1B-2c_3d"); got != "bvp0a1b2c3d" {
		t.Fatalf("RemoteName = %q", got)
	}
	for repo, want := range map[string]string{
		"rclone:" + RemoteName("0a1b2c") + ":bombvault/container": "0a1b2c",
		"rclone:" + RemoteName("0a1b2c") + ":":                    "0a1b2c",
		"rclone:gdrive:bombvault":                                 "",
		"rclone:bvp0a1b":                                          "",
		"s3:https://s3.example.com/bvp":                           "",
	} {
		if got := RemotePlace(repo); got != want {
			t.Errorf("RemotePlace(%q) = %q, want %q", repo, got, want)
		}
	}
}

func TestEnvRendersEachKind(t *testing.T) {
	pass, err := Obscure("app-pass")
	if err != nil {
		t.Fatal(err)
	}
	c := Creds{
		S3KeyID: "K", S3Secret: "S", S3Region: "auto", RESTUser: "u", RESTPassword: "p",
		WebDAVURL: "https://cloud.example.com/remote.php/dav/files/anna/", WebDAVVendor: "nextcloud", WebDAVUser: "anna", WebDAVPass: "app-pass",
		AzureAccount: "acct", AzureKey: "key",
	}
	for _, tc := range []struct {
		kind Kind
		want []string
	}{
		{KindS3, []string{"AWS_ACCESS_KEY_ID=K", "AWS_SECRET_ACCESS_KEY=S", "AWS_DEFAULT_REGION=auto"}},
		{KindREST, []string{"RESTIC_REST_USERNAME=u", "RESTIC_REST_PASSWORD=p"}},
		{KindWebDAV, []string{
			"RCLONE_CONFIG_BVP0A1B_TYPE=webdav",
			"RCLONE_CONFIG_BVP0A1B_URL=https://cloud.example.com/remote.php/dav/files/anna/",
			"RCLONE_CONFIG_BVP0A1B_VENDOR=nextcloud",
			"RCLONE_CONFIG_BVP0A1B_USER=anna",
			"RCLONE_CONFIG_BVP0A1B_PASS=" + pass,
		}},
		{KindAzure, []string{"AZURE_ACCOUNT_NAME=acct", "AZURE_ACCOUNT_KEY=key"}},
		{KindLocal, nil}, {KindSFTP, nil}, {KindRclone, nil},
	} {
		got, err := Env(tc.kind, c, "0a1b")
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("Env(%s) = %v, %v\nwant %v", tc.kind, got, err, tc.want)
		}
	}
	if got, _ := Env(KindS3, Creds{S3KeyID: "K"}, ""); !slices.Equal(got, []string{"AWS_ACCESS_KEY_ID=K"}) {
		t.Errorf("an unset value became a variable: %v", got)
	}
}

func TestCollidesOnlyOnOneVariableWithTwoValues(t *testing.T) {
	a := []string{"AWS_ACCESS_KEY_ID=A", "RESTIC_REST_USERNAME=u"}
	for _, c := range []struct {
		b    []string
		want bool
	}{
		{[]string{"AWS_ACCESS_KEY_ID=B"}, true},
		{[]string{"AWS_ACCESS_KEY_ID=A"}, false},
		{[]string{"RCLONE_CONFIG_BVP1_TYPE=webdav"}, false},
		{nil, false},
	} {
		if got := Collides(a, c.b); got != c.want {
			t.Errorf("Collides(%v, %v) = %v", a, c.b, got)
		}
	}
	if !Collides([]string{"X=1=2"}, []string{"X=1=3"}) {
		t.Error("a value holding '=' is compared whole")
	}
}

func TestNeededKeepsWhatTheBackendReads(t *testing.T) {
	env := []string{"AWS_ACCESS_KEY_ID=A", "RESTIC_REST_USERNAME=u", "RCLONE_CONFIG_BVP1_TYPE=webdav", "AZURE_ACCOUNT_NAME=n"}
	for repo, want := range map[string][]string{
		"s3:https://s3.example.com/bv": {"AWS_ACCESS_KEY_ID=A"},
		"rest:http://nas:8000/bv":      {"RESTIC_REST_USERNAME=u"},
		"rclone:bvp1:bv":               {"RCLONE_CONFIG_BVP1_TYPE=webdav"},
		"azure:c:/bv":                  {"AZURE_ACCOUNT_NAME=n"},
		"sftp://bv@box:22/bv":          nil,
		"b2:bucket:bv":                 nil,
		"user/bombvault/containers":    nil,
	} {
		if got := Needed(repo, env); !slices.Equal(got, want) {
			t.Errorf("Needed(%q) = %v, want %v", repo, got, want)
		}
	}
}
