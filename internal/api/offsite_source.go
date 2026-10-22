package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// sourceRemote is the rclone remote a source is read through when its
// credentials would take the destination's environment variables.
const sourceRemote = "BVSRC"

// sourceAccess is how one restic process reaches a source with its own
// credentials while it holds the destination's in destMode. The answer adds
// what the source needs to a copy of that environment, or, where both would
// claim the same variables with different values, a location that needs none:
// an S3 source is read through an rclone remote defined in the environment, a
// rest-server source with its login in the address.
//
// The source's credentials are the ones its backups run with, so a remote
// primary or a named repository on the shared set is read the way it was
// written. A local source, or one with no credentials stored, is read as it is.
func (s *Service) sourceAccess(settings store.Settings, domain string, src domainRepoRef, dest string, destMode restic.Mode) (string, restic.Mode, error) {
	if !restic.IsRemoteRepo(src.Loc) {
		return src.Loc, destMode, nil
	}
	c := credsInEnv(s.primaryModeFor(settings, domain, src.Loc).Env)
	mode := destMode
	mode.Env = slices.Clone(destMode.Env)
	srcScheme, destScheme := repoScheme(src.Loc), repoScheme(dest)
	switch srcScheme {
	case "s3":
		if c.S3KeyID == "" {
			return src.Loc, mode, nil
		}
		if destScheme != "s3" {
			mode.Env = append(mode.Env, cloudEnv(CloudCreds{S3KeyID: c.S3KeyID, S3Secret: c.S3Secret, S3Region: c.S3Region})...)
			return src.Loc, mode, nil
		}
		if envValue(destMode.Env, "AWS_ACCESS_KEY_ID") == c.S3KeyID && envValue(destMode.Env, "AWS_SECRET_ACCESS_KEY") == c.S3Secret {
			return src.Loc, mode, nil
		}
		loc, env, err := s3ThroughRclone(src.Loc, c)
		if err != nil {
			return "", restic.Mode{}, err
		}
		mode.Env = append(mode.Env, env...)
		return loc, mode, nil
	case "rest":
		if c.RESTUser == "" && c.RESTPassword == "" {
			return src.Loc, mode, nil
		}
		if destScheme != "rest" {
			mode.Env = append(mode.Env, cloudEnv(CloudCreds{RESTUser: c.RESTUser, RESTPassword: c.RESTPassword})...)
			return src.Loc, mode, nil
		}
		if envValue(destMode.Env, "RESTIC_REST_USERNAME") == c.RESTUser && envValue(destMode.Env, "RESTIC_REST_PASSWORD") == c.RESTPassword {
			return src.Loc, mode, nil
		}
		loc, err := restWithLogin(src.Loc, c.RESTUser, c.RESTPassword)
		return loc, mode, err
	}
	return src.Loc, mode, nil
}

// credsInEnv reads back the credentials cloudEnv put into an environment.
func credsInEnv(env []string) CloudCreds {
	return CloudCreds{
		S3KeyID:      envValue(env, "AWS_ACCESS_KEY_ID"),
		S3Secret:     envValue(env, "AWS_SECRET_ACCESS_KEY"),
		S3Region:     envValue(env, "AWS_DEFAULT_REGION"),
		RESTUser:     envValue(env, "RESTIC_REST_USERNAME"),
		RESTPassword: envValue(env, "RESTIC_REST_PASSWORD"),
	}
}

// repoScheme is the restic backend a location names, "" for a path.
func repoScheme(loc string) string {
	scheme, _, ok := strings.Cut(strings.TrimSpace(loc), ":")
	if !ok || strings.ContainsAny(scheme, `/\`) {
		return ""
	}
	return strings.ToLower(scheme)
}

// s3ThroughRclone turns an S3 location into the same bucket and path behind
// an rclone remote that lives only in the environment of one restic process.
func s3ThroughRclone(loc string, c CloudCreds) (string, []string, error) {
	rest := strings.TrimPrefix(strings.TrimSpace(loc), "s3:")
	if !strings.Contains(rest, "://") {
		rest = "https://" + rest
	}
	u, err := url.Parse(rest)
	if err != nil || u.Host == "" {
		return "", nil, fmt.Errorf("%s is not an S3 location rclone can read", shortRepoName(loc))
	}
	path := strings.Trim(u.Path, "/")
	if path == "" {
		return "", nil, fmt.Errorf("%s names no bucket", shortRepoName(loc))
	}
	prefix := "RCLONE_CONFIG_" + sourceRemote + "_"
	env := []string{
		prefix + "TYPE=s3",
		prefix + "PROVIDER=Other",
		prefix + "ACCESS_KEY_ID=" + c.S3KeyID,
		prefix + "SECRET_ACCESS_KEY=" + c.S3Secret,
		prefix + "ENDPOINT=" + u.Scheme + "://" + u.Host,
	}
	region := c.S3Region
	if region == "" {
		region = awsRegionOf(u.Hostname())
	}
	if region == "" && isGlobalAWSHost(u.Hostname()) {
		bucket, _, _ := strings.Cut(path, "/")
		region = bucketRegion(u.Scheme+"://"+u.Host, bucket)
	}
	if region != "" {
		env = append(env, prefix+"REGION="+region)
	}
	return "rclone:" + sourceRemote + ":" + path, env, nil
}

// awsRegionHostRe matches a regional AWS S3 endpoint and captures its region:
// s3.eu-central-1.amazonaws.com, s3-eu-west-1.amazonaws.com, the dualstack form
// and the .com.cn hosts of the China regions.
var awsRegionHostRe = regexp.MustCompile(`^(?:[^.]+\.)?s3[.-](?:dualstack\.)?([a-z]{2}(?:-[a-z]+)+-\d+)\.amazonaws\.com(?:\.cn)?$`)

// awsRegionOf is the region a regional AWS S3 endpoint names, "" for any other
// host. restic finds the region of such a bucket by itself, while rclone signs
// for us-east-1 without one and AWS refuses every request.
func awsRegionOf(host string) string {
	if m := awsRegionHostRe.FindStringSubmatch(strings.ToLower(host)); m != nil {
		return m[1]
	}
	return ""
}

// isGlobalAWSHost reports whether host is AWS's endpoint without a region,
// which a location gets when its sign-in names none.
func isGlobalAWSHost(host string) bool {
	host = strings.ToLower(host)
	return host == "s3.amazonaws.com" || host == "s3-external-1.amazonaws.com"
}

// bucketRegion asks AWS where a bucket lives. AWS names the region in the
// X-Amz-Bucket-Region header of every answer to a HEAD of the bucket, even one
// that refuses it, so no sign-in is needed. A var so a test answers without
// the network.
var bucketRegion = func(endpoint, bucket string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, strings.TrimRight(endpoint, "/")+"/"+url.PathEscape(bucket), nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	_ = resp.Body.Close()
	return resp.Header.Get("X-Amz-Bucket-Region")
}

// restWithLogin puts a login into a rest-server address, which restic reads
// before RESTIC_REST_USERNAME and RESTIC_REST_PASSWORD.
func restWithLogin(loc, user, pass string) (string, error) {
	u, err := url.Parse(strings.TrimPrefix(strings.TrimSpace(loc), "rest:"))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("%s is not a rest-server address", shortRepoName(loc))
	}
	if user != "" {
		u.User = url.UserPassword(user, pass)
	}
	return "rest:" + u.String(), nil
}
