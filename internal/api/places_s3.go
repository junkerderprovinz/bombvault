package api

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// placeProbeHTTPClient carries the probes' own requests to a provider's API;
// the timeout backstops each request's context.
var placeProbeHTTPClient = &http.Client{Timeout: 25 * time.Second}

// emptySHA256 is the payload hash of a request without a body.
const emptySHA256 = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

// signV4 signs a bodiless S3 request with AWS Signature Version 4, over the
// host and every header the request carries.
func signV4(req *http.Request, keyID, secret, region string, now time.Time) {
	amzDate := now.UTC().Format("20060102T150405Z")
	day := amzDate[:8]
	req.Header.Set("X-Amz-Date", amzDate)
	req.Header.Set("X-Amz-Content-Sha256", emptySHA256)

	headers := map[string]string{"host": req.URL.Host}
	for key, values := range req.Header {
		headers[strings.ToLower(key)] = strings.TrimSpace(strings.Join(values, ","))
	}
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	slices.Sort(names)
	var canonical strings.Builder
	for _, name := range names {
		canonical.WriteString(name + ":" + headers[name] + "\n")
	}
	signed := strings.Join(names, ";")
	path := req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	query := strings.ReplaceAll(req.URL.Query().Encode(), "+", "%20")
	request := strings.Join([]string{req.Method, path, query, canonical.String(), signed, emptySHA256}, "\n")
	scope := day + "/" + region + "/s3/aws4_request"
	sum := sha256.Sum256([]byte(request))
	toSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(sum[:])
	key := hmacSHA256([]byte("AWS4"+secret), day)
	for _, part := range []string{region, "s3", "aws4_request"} {
		key = hmacSHA256(key, part)
	}
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+keyID+"/"+scope+
		", SignedHeaders="+signed+", Signature="+hex.EncodeToString(hmacSHA256(key, toSign)))
}

func hmacSHA256(key []byte, data string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(data))
	return m.Sum(nil)
}

// errBucketsHidden is a key that works but may not list buckets, which is how
// most keys limited to one bucket answer.
var errBucketsHidden = errors.New("this key may not list buckets")

// s3ListBuckets lists the buckets a key can see with one signed ListBuckets
// request. Only a refused key or an endpoint that does not answer fail the
// probe; every other refusal leaves the bucket to be typed, and restic's own
// probe of it decides.
func s3ListBuckets(ctx context.Context, endpoint string, c places.Creds) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errPlaceProbeFailed, err)
	}
	signV4(req, c.S3KeyID, c.S3Secret, cmp.Or(c.S3Region, "us-east-1"), time.Now())
	resp, err := placeProbeHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: the endpoint did not answer: %v", errPlaceProbeFailed, err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: the endpoint's answer could not be read: %v", errPlaceProbeFailed, err)
	}
	if resp.StatusCode == http.StatusOK {
		var list struct {
			Names []string `xml:"Buckets>Bucket>Name"`
		}
		if err := xml.Unmarshal(body, &list); err != nil {
			return nil, fmt.Errorf("%w: the bucket list could not be read: %v", errPlaceProbeFailed, err)
		}
		return list.Names, nil
	}
	var refusal struct {
		Code string `xml:"Code"`
	}
	_ = xml.Unmarshal(body, &refusal)
	switch refusal.Code {
	case "InvalidAccessKeyId", "SignatureDoesNotMatch":
		return nil, fmt.Errorf("%w: the storage refused this key (%s)", errPlaceProbeFailed, refusal.Code)
	}
	return nil, errBucketsHidden
}

// listBucketsInto offers the buckets the key can see and names a typed bucket
// that is not among them. A key that may not list is no failure; the form then
// asks for the bucket.
func listBucketsInto(ctx context.Context, res *places.ProbeResult, endpoint string, c places.Creds, typed string) error {
	names, err := s3ListBuckets(ctx, endpoint, c)
	switch {
	case errors.Is(err, errBucketsHidden):
		res.Facts = append(res.Facts, places.ProbeFact{Key: places.FactBucketsHidden})
		return nil
	case err != nil:
		return err
	}
	res.Buckets = names
	if bucket := strings.TrimSpace(typed); bucket != "" && !slices.Contains(names, bucket) {
		res.Facts = append(res.Facts, places.ProbeFact{Key: places.FactBucketNew, Params: map[string]string{"bucket": bucket}})
	}
	return nil
}

// probeS3 lists what the key can see before the address is built.
func probeS3(ctx context.Context, pr *placeProbe, res *places.ProbeResult) error {
	endpoint, err := places.Endpoint(pr.provider, pr.fields)
	if err != nil {
		return err
	}
	return listBucketsInto(ctx, res, endpoint, pr.creds, pr.fields["bucket"])
}
