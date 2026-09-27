package api

import "strings"

// App-specific advisories for the exclusion assistant.
//
// The assistant answers a size question: which folders under this container are
// big and regenerable. Some of the most expensive backup mistakes are not size
// questions, and no folder scan can ever see them. This table carries those.
//
// The case it exists for: Immich stores every photo's metadata in a PostgreSQL
// database running in a SEPARATE container. A perfect file-level backup of the
// Immich container therefore restores the pictures and loses the albums, the
// faces, the dates and the sharing. Nothing about the folder sizes hints at
// that, the restore looks like it worked, and the loss is discovered later.
//
// Advisories are IDs, never English prose. The interface maps an id to a
// translation key, so the text lives in the 42 locale tables where every other
// user-facing sentence lives, and an id from a newer server that the interface
// does not know yet renders nothing instead of a raw literal. liveReason
// already works exactly this way.
const (
	// advisoryImmichSeparateDB: the metadata is in a Postgres container of its
	// own and needs its own backup.
	advisoryImmichSeparateDB = "immich-db-separate"
	// advisoryNextcloudSeparateDB: same shape. A Nextcloud install's files are
	// on disk, but the shares, tags and accounts are in its database.
	advisoryNextcloudSeparateDB = "nextcloud-db-separate"
)

// appAdvisory is one rule: which images it applies to, and what it says.
type appAdvisory struct {
	// match is compared against the normalised image REPOSITORY (registry host
	// kept, tag and digest stripped, lower-cased) as a substring, so a fork or
	// a private mirror of the same image still matches.
	match []string
	id    string
}

// appAdvisories is kept short. Every entry is a claim about how an
// application stores its data, which can go out of date, so this holds only
// cases where being wrong about it costs data rather than disk space.
var appAdvisories = []appAdvisory{
	{
		match: []string{"immich-app/immich-server", "imagegenius/immich"},
		id:    advisoryImmichSeparateDB,
	},
	{
		match: []string{"nextcloud", "linuxserver/nextcloud"},
		id:    advisoryNextcloudSeparateDB,
	},
}

// appAdvisoriesFor returns the advisory ids for one image reference.
//
// An empty or unknown image yields nothing, which is what keeps the assistant's
// existing behaviour unchanged for every container not in the table.
func appAdvisoriesFor(image string) []string {
	repo := normalizeImageRepo(image)
	if repo == "" {
		return nil
	}
	var out []string
	for _, rule := range appAdvisories {
		for _, m := range rule.match {
			if strings.Contains(repo, m) {
				out = append(out, rule.id)
				break
			}
		}
	}
	return out
}

// normalizeImageRepo reduces an image reference to its comparable repository
// part: lower-cased, with the digest and the tag removed.
//
// The tag has to go or a rule would only fire on whichever tag it was written
// against, missing every pinned install, which is most of them. The digest has
// to go for the same reason. Splitting the tag off is done AFTER the digest so
// "repo:tag@sha256:…" reduces correctly, and only on the last path segment, so
// a registry port ("registry.example:5000/app") is not mistaken for a tag.
func normalizeImageRepo(image string) string {
	ref := strings.ToLower(strings.TrimSpace(image))
	if ref == "" {
		return ""
	}
	if i := strings.Index(ref, "@"); i >= 0 {
		ref = ref[:i]
	}
	slash := strings.LastIndex(ref, "/")
	if i := strings.LastIndex(ref, ":"); i > slash {
		ref = ref[:i]
	}
	return ref
}
