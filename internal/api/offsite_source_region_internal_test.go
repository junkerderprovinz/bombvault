package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// A sign-in may leave the region empty, since restic finds it for an AWS
// bucket. rclone does not, so the region comes from the endpoint.
func TestAnAWSSourceWithoutARegionTakesItFromTheEndpoint(t *testing.T) {
	svc, st, _ := newProbeSvc(t, nil)
	src := namedSource(t, svc, st, "s3:s3.eu-central-1.amazonaws.com/bucket/bv", CloudCreds{S3KeyID: "SRC", S3Secret: "srcsecret"})
	settings, _ := st.GetSettings()
	dest := restic.Mode{Env: []string{"AWS_ACCESS_KEY_ID=DEST", "AWS_SECRET_ACCESS_KEY=destsecret"}}

	_, mode, err := svc.sourceAccess(settings, "containers", src, "s3:https://s3.example.com/other", dest)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(mode.Env, "RCLONE_CONFIG_BVSRC_REGION=eu-central-1") {
		t.Fatalf("env = %v, want the region eu-central-1", mode.Env)
	}
}

func TestTheRegionOfAnAWSEndpoint(t *testing.T) {
	for host, want := range map[string]string{
		"s3.eu-central-1.amazonaws.com":            "eu-central-1",
		"S3-EU-WEST-1.amazonaws.com":               "eu-west-1",
		"s3.dualstack.us-gov-west-1.amazonaws.com": "us-gov-west-1",
		"bucket.s3.ap-southeast-2.amazonaws.com":   "ap-southeast-2",
		"s3.cn-north-1.amazonaws.com.cn":           "cn-north-1",
		"s3.amazonaws.com":                         "",
		"s3-external-1.amazonaws.com":              "",
		"s3.eu-central-2.wasabisys.com":            "",
		"minio.local":                              "",
	} {
		if got := awsRegionOf(host); got != want {
			t.Errorf("awsRegionOf(%q) = %q, want %q", host, got, want)
		}
	}
}
