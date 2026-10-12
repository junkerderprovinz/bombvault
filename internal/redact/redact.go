// Package redact takes credentials out of text that names a storage location,
// so that a location, or a name somebody typed one into, can be shown, sent
// or logged.
package redact

import "regexp"

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
	return RcloneParams(userinfoRe.ReplaceAllString(s, Marker+"@"))
}

// RcloneParams replaces the value of every parameter of an rclone connection
// string in s with Marker.
func RcloneParams(s string) string {
	return rcloneParamRe.ReplaceAllString(s, ",${1}="+Marker)
}
