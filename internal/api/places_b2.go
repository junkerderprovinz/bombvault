package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// b2AuthorizeURL is Backblaze's sign-in for a key, a var so a test can point it
// at a stand-in.
var b2AuthorizeURL = "https://api.backblazeb2.com/b2api/v4/b2_authorize_account"

// b2Authorization is the part of b2_authorize_account's answer a probe reads.
// Buckets is null for a key that may reach every bucket of the account.
type b2Authorization struct {
	APIInfo struct {
		StorageAPI struct {
			S3APIURL string `json:"s3ApiUrl"`
			Allowed  struct {
				Buckets []struct {
					Name *string `json:"name"`
				} `json:"buckets"`
				NamePrefix *string `json:"namePrefix"`
			} `json:"allowed"`
		} `json:"storageApi"`
	} `json:"apiInfo"`
}

// b2Authorize signs a key in at Backblaze. A refused key and a Backblaze that
// does not answer are failed probes.
func b2Authorize(ctx context.Context, keyID, key string) (b2Authorization, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b2AuthorizeURL, nil)
	if err != nil {
		return b2Authorization{}, fmt.Errorf("%w: %v", errPlaceProbeFailed, err)
	}
	req.SetBasicAuth(keyID, key)
	resp, err := placeProbeHTTPClient.Do(req)
	if err != nil {
		return b2Authorization{}, fmt.Errorf("%w: Backblaze did not answer: %v", errPlaceProbeFailed, err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return b2Authorization{}, fmt.Errorf("%w: Backblaze refused this key ID and application key", errPlaceProbeFailed)
	case resp.StatusCode != http.StatusOK:
		return b2Authorization{}, fmt.Errorf("%w: Backblaze answered HTTP %d", errPlaceProbeFailed, resp.StatusCode)
	}
	var auth b2Authorization
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&auth); err != nil {
		return b2Authorization{}, fmt.Errorf("%w: Backblaze's answer could not be read: %v", errPlaceProbeFailed, err)
	}
	if auth.APIInfo.StorageAPI.S3APIURL == "" {
		return b2Authorization{}, fmt.Errorf("%w: Backblaze named no S3 endpoint for this key", errPlaceProbeFailed)
	}
	return auth, nil
}

// probeB2 lets Backblaze say what the key reaches, so nobody types an endpoint
// or a bucket: b2_authorize_account names the S3 endpoint, the buckets a
// restricted key is limited to and the folder it may write under.
func probeB2(ctx context.Context, pr *placeProbe, res *places.ProbeResult) error {
	auth, err := b2Authorize(ctx, pr.creds.S3KeyID, pr.creds.S3Secret)
	if err != nil {
		return err
	}
	storage := auth.APIInfo.StorageAPI
	pr.fields["endpoint"] = storage.S3APIURL
	pr.creds.S3Region = places.S3Region(pr.provider, pr.fields)
	if storage.Allowed.NamePrefix != nil {
		if prefix := strings.Trim(*storage.Allowed.NamePrefix, "/"); prefix != "" {
			pr.fields["path"] = underPrefix(prefix, pr.fields["path"])
			res.Facts = append(res.Facts, places.ProbeFact{Key: places.FactB2Prefix, Params: map[string]string{"prefix": prefix}})
		}
	}
	if storage.Allowed.Buckets == nil {
		return listBucketsInto(ctx, res, storage.S3APIURL, pr.creds, pr.fields["bucket"])
	}
	var names []string
	for _, b := range storage.Allowed.Buckets {
		if b.Name != nil {
			names = append(names, *b.Name)
		}
	}
	bucket := strings.TrimSpace(pr.fields["bucket"])
	switch {
	case bucket != "" && !slices.Contains(names, bucket):
		return fmt.Errorf("%w: this key is limited to other buckets", errPlaceProbeFailed)
	case bucket == "" && len(names) == 1:
		pr.fields["bucket"] = names[0]
		res.Facts = append(res.Facts, places.ProbeFact{Key: places.FactB2Bucket, Params: map[string]string{"bucket": names[0]}})
	default:
		res.Buckets = names
	}
	return nil
}

// underPrefix puts path below the folder a key is limited to, unless it lies
// there already.
func underPrefix(prefix, path string) string {
	path = strings.Trim(strings.TrimSpace(path), "/")
	switch {
	case path == "":
		return prefix
	case path == prefix, strings.HasPrefix(path, prefix+"/"):
		return path
	}
	return prefix + "/" + path
}
