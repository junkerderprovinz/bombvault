package api

import (
	"cmp"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/bombvault/internal/places"
)

// azureBlobURL is the Blob service of a storage account, a var so a test can
// point it at a stand-in.
var azureBlobURL = "https://{account}.blob.core.windows.net/"

// azureAPIVersion is the Blob service version the listing asks for.
const azureAPIVersion = "2025-05-05"

// azureAccountRe is Azure's rule for a storage account name, which also keeps
// the name from reaching into the host part of azureBlobURL.
var azureAccountRe = regexp.MustCompile(`^[a-z0-9]{3,24}$`)

// signSharedKey signs a bodiless Blob service request with the account key in
// Azure's Shared Key scheme, over the x-ms- headers the request carries. The
// eleven standard headers the scheme lists stay empty for a GET without a body.
func signSharedKey(req *http.Request, account, key string) error {
	secret, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("%w: the access key is not valid: %v", errPlaceProbeFailed, err)
	}
	var names []string
	for name := range req.Header {
		if lower := strings.ToLower(name); strings.HasPrefix(lower, "x-ms-") {
			names = append(names, lower)
		}
	}
	slices.Sort(names)
	var b strings.Builder
	b.WriteString(req.Method + strings.Repeat("\n", 12))
	for _, name := range names {
		b.WriteString(name + ":" + strings.TrimSpace(req.Header.Get(name)) + "\n")
	}
	b.WriteString("/" + account + cmp.Or(req.URL.EscapedPath(), "/"))
	query := req.URL.Query()
	for _, name := range slices.Sorted(maps.Keys(query)) {
		values := slices.Sorted(slices.Values(query[name]))
		b.WriteString("\n" + strings.ToLower(name) + ":" + strings.Join(values, ","))
	}
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(b.String()))
	req.Header.Set("Authorization", "SharedKey "+account+":"+base64.StdEncoding.EncodeToString(m.Sum(nil)))
	return nil
}

// azureListContainers lists the containers of a storage account with one
// signed List Containers request. An account key may read the whole account,
// so every refusal fails the probe.
func azureListContainers(ctx context.Context, account, key string) ([]string, error) {
	if !azureAccountRe.MatchString(account) {
		return nil, fmt.Errorf("%w: a storage account name is 3 to 24 lower-case letters and digits", errPlaceProbeFailed)
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	endpoint := strings.ReplaceAll(azureBlobURL, "{account}", account)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?comp=list", nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errPlaceProbeFailed, err)
	}
	req.Header.Set("x-ms-date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("x-ms-version", azureAPIVersion)
	if err := signSharedKey(req, account, key); err != nil {
		return nil, err
	}
	resp, err := placeProbeHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: the storage account did not answer: %v", errPlaceProbeFailed, err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, fmt.Errorf("%w: the storage account's answer could not be read: %v", errPlaceProbeFailed, err)
	}
	if resp.StatusCode != http.StatusOK {
		var refusal struct {
			Code string `xml:"Code"`
		}
		_ = xml.Unmarshal(body, &refusal)
		return nil, fmt.Errorf("%w: Azure refused the listing (HTTP %d %s)", errPlaceProbeFailed, resp.StatusCode, refusal.Code)
	}
	var list struct {
		Names []string `xml:"Containers>Container>Name"`
	}
	if err := xml.Unmarshal(body, &list); err != nil {
		return nil, fmt.Errorf("%w: the container list could not be read: %v", errPlaceProbeFailed, err)
	}
	return list.Names, nil
}

// probeAzure offers the containers of the account while the form names none.
func probeAzure(ctx context.Context, pr *placeProbe, res *places.ProbeResult) error {
	switch {
	case pr.creds.AzureAccount == "":
		return places.MissingField("account")
	case pr.creds.AzureKey == "":
		return places.MissingField("secret")
	}
	names, err := azureListContainers(ctx, pr.creds.AzureAccount, pr.creds.AzureKey)
	if err != nil {
		return err
	}
	res.Buckets = names
	return nil
}
