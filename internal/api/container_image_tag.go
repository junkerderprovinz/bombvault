package api

import (
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/model"
)

// imageTagPrefix marks the image a container ran when its snapshot was taken.
// The stored definition holds only the image of the newest backup, so a
// restore of an older snapshot reads the image from the snapshot itself.
const imageTagPrefix = "bv:image:"

// imageTags names the image in runs on its backup, nothing when Docker gave
// it no id.
func imageTags(in model.Inspect) []string {
	if in.Image == "" {
		return nil
	}
	return []string{imageTagPrefix + in.Image}
}

// taggedImage is the image a snapshot's tags name, "" for a snapshot taken
// before snapshots carried one.
func taggedImage(tags []string) string {
	for _, t := range tags {
		if id, ok := strings.CutPrefix(t, imageTagPrefix); ok {
			return id
		}
	}
	return ""
}
