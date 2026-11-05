// Package redact takes credentials out of text that names a storage location,
// so that a location, or a name somebody typed one into, can be shown, sent
// or logged.
package redact

import (
	"cmp"
	"regexp"
	"strings"
)

// Marker stands in for what was taken out.
const Marker = "[redacted]"

// userinfoRe is the user:password@ of a URL. The whole match goes, so a
// quote in the password ends nothing.
var userinfoRe = regexp.MustCompile(`[\w.+%-]*:[^\s/@]+@`)

// rcloneParamRe is one key=value parameter of an rclone connection string
// such as rclone::s3,access_key_id=...,secret_access_key=...:bucket. A value
// that holds a comma or a colon is quoted, a quote inside it doubled.
var rcloneParamRe = regexp.MustCompile(`,([\w-]+)=("(?:[^"]|"")*"|'(?:[^']|'')*'|[^,:]*)`)

// Credentials replaces the user:password@ of every URL in s and the value of
// every parameter of an rclone connection string with Marker, and leaves
// scheme, host, bucket and path.
func Credentials(s string) string {
	return RcloneParams(userinfoRe.ReplaceAllStringFunc(s, maskUserinfo))
}

// maskUserinfo masks one match of userinfoRe. In sftp:user@host:path the colon
// belongs to the scheme, so that form holds no password and keeps its name.
func maskUserinfo(match string) string {
	user, ok := strings.CutPrefix(match, "sftp:")
	if !ok {
		return Marker + "@"
	}
	if strings.Contains(user, ":") {
		return "sftp:" + Marker + "@"
	}
	return match
}

// RcloneParams replaces the value of every parameter of an rclone connection
// string in s with Marker.
func RcloneParams(s string) string {
	return rcloneParamRe.ReplaceAllString(s, ",${1}="+Marker)
}

// Restore turns edited, a location Credentials masked and somebody then
// changed, back into one that holds the credentials of stored. It reports false
// when edited names another place than those credentials belong to: a path may
// change under a hidden password, but the password must not follow a new host to
// a server nobody typed it for.
func Restore(edited, stored string) (string, bool) {
	masked := Credentials(stored)
	if edited == masked {
		return stored, true
	}
	place, ok := signInPlace(masked)
	if !ok {
		return "", false
	}
	newTail, ok := strings.CutPrefix(edited, place)
	if !ok {
		return "", false
	}
	oldTail := masked[len(place):]
	// What follows has to start like it did, or "box:8000" would also accept
	// "box:80001" and "box.example.com".
	if newTail != "" && newTail[0] != cmp.Or(oldTail, "/")[0] {
		return "", false
	}
	return stored[:len(stored)-len(oldTail)] + newTail, true
}

// signInPlace is the start of a masked location, up to and including what its
// last credential signs in to: the host after a user:password@, or nothing more
// after a parameter of an rclone connection string.
func signInPlace(masked string) (string, bool) {
	i := strings.LastIndex(masked, Marker)
	if i < 0 {
		return "", false
	}
	end := i + len(Marker)
	if rest := masked[end:]; strings.HasPrefix(rest, "@") {
		if slash := strings.Index(rest, "/"); slash >= 0 {
			return masked[:end+slash], true
		}
		return masked, true
	}
	return masked[:end], true
}
