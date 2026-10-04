package api

import (
	"fmt"
	"net/url"
	"slices"
	"strings"

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
// The domain's own repository and a source without a credential set of its
// own are read as they are.
func (s *Service) sourceAccess(settings store.Settings, src domainRepoRef, dest string, destMode restic.Mode) (string, restic.Mode, error) {
	if src.Own || strings.TrimSpace(src.Named.CredsRef) == "" || !restic.IsRemoteRepo(src.Loc) {
		return src.Loc, destMode, nil
	}
	c, err := s.decodeCloudFor(settings, src.Named.CredsRef)
	if err != nil {
		return "", restic.Mode{}, fmt.Errorf("read the credentials of %s: %w", shortRepoName(src.Loc), err)
	}
	mode := destMode
	mode.Env = slices.Clone(destMode.Env)
	srcScheme, destScheme := repoScheme(src.Loc), repoScheme(dest)
	switch srcScheme {
	case "s3":
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
	if c.S3Region != "" {
		env = append(env, prefix+"REGION="+c.S3Region)
	}
	return "rclone:" + sourceRemote + ":" + path, env, nil
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
