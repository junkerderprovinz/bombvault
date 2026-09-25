package api

import (
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/places"
	"github.com/junkerderprovinz/bombvault/internal/restic"
)

// placeKindsByScheme are the remote schemes a place can hold. b2:, gs:,
// swift: and azure: addresses take their keys from the container's
// environment, which no place knows about, so they stay without a place.
var placeKindsByScheme = map[string]places.Kind{
	"s3": places.KindS3, "rest": places.KindREST, "sftp": places.KindSFTP, "rclone": places.KindRclone,
}

// placeKindOf is the kind of place that can hold loc. A word and a colon that
// restic does not know is a mistyped remote, not a local folder.
func placeKindOf(loc string) (places.Kind, bool) {
	if strings.TrimSpace(loc) == "" || restic.LooksLikeUnprefixedRemote(loc) {
		return "", false
	}
	if !restic.IsRemoteRepo(loc) {
		return places.KindLocal, true
	}
	scheme, _, _ := strings.Cut(loc, ":")
	kind, ok := placeKindsByScheme[scheme]
	return kind, ok
}

// addressRoot is the part of loc that names its store and holds no folder:
// scheme, host and bucket of an s3 address, the server of a rest: or sftp://
// address, the host or remote with its colon in the colon forms, and nothing
// for a local path.
func addressRoot(kind places.Kind, loc string) (string, bool) {
	scheme, rest, _ := strings.Cut(loc, ":")
	switch kind {
	case places.KindLocal:
		return "", true
	case places.KindS3, places.KindREST:
		after := strings.TrimPrefix(strings.TrimPrefix(rest, "https://"), "http://")
		host, path, _ := strings.Cut(after, "/")
		if host == "" {
			return "", false
		}
		end := len(loc) - len(after) + len(host)
		if kind == places.KindS3 {
			bucket, _, _ := strings.Cut(path, "/")
			if bucket == "" {
				return "", false
			}
			end += 1 + len(bucket)
		}
		return loc[:end], true
	case places.KindSFTP, places.KindRclone:
		if url, ok := strings.CutPrefix(rest, "//"); ok {
			host, _, _ := strings.Cut(url, "/")
			if host == "" {
				return "", false
			}
			return loc[:len(loc)-len(url)+len(host)], true
		}
		host, _, found := strings.Cut(rest, ":")
		if !found || host == "" {
			return "", false
		}
		return loc[:len(scheme)+1+len(host)+1], true
	}
	return "", false
}

// placeSplit is an address read as a place and one folder under it. An empty
// folder means the address is the root of its bucket or server and so a
// repository of its own.
type placeSplit struct {
	kind   places.Kind
	base   string
	folder string
}

// splitPlaceAddress reads loc as a base and its last path element, and only
// when places.Join spells loc back byte for byte: a trailing slash, a folder
// right after "sftp:host:/", a local path without a parent or a scheme no
// place has leaves loc unsplit.
func splitPlaceAddress(loc string) (placeSplit, bool) {
	kind, ok := placeKindOf(loc)
	if !ok {
		return placeSplit{}, false
	}
	root, ok := addressRoot(kind, loc)
	if !ok {
		return placeSplit{}, false
	}
	if len(pathElements(loc[len(root):])) == 0 {
		if kind == places.KindLocal {
			return placeSplit{}, false
		}
		return placeSplit{kind: kind, base: loc}, true
	}
	base, folder := root, loc[len(root):]
	if i := strings.LastIndexByte(loc, '/'); i >= len(root) {
		base, folder = loc[:i], loc[i+1:]
	}
	if base == "" || folder == "" || places.Join(base, folder) != loc {
		return placeSplit{}, false
	}
	return placeSplit{kind: kind, base: base, folder: folder}, true
}
