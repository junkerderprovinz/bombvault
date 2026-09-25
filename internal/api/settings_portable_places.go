package api

import "github.com/junkerderprovinz/bombvault/internal/store"

// placeExport is a storage place in the settings file: every column but the
// secrets, which travel in the credentials block under the set CredsRef names.
type placeExport struct {
	ID                   string            `json:"id"`
	Name                 string            `json:"name"`
	Provider             string            `json:"provider"`
	Kind                 string            `json:"kind"`
	Base                 string            `json:"base"`
	Folders              map[string]string `json:"folders"`
	CredsRef             string            `json:"credsRef"`
	OffPremises          bool              `json:"offPremises"`
	StorageClass         string            `json:"storageClass"`
	Immutable            bool              `json:"immutable"`
	RetentionKeepLast    int               `json:"retentionKeepLast"`
	RetentionKeepDaily   int               `json:"retentionKeepDaily"`
	RetentionKeepWeekly  int               `json:"retentionKeepWeekly"`
	RetentionKeepMonthly int               `json:"retentionKeepMonthly"`
	LimitUpload          int               `json:"limitUpload"`
	LimitDownload        int               `json:"limitDownload"`
	GrowthBudgetGB       int               `json:"growthBudgetGb"`
	Enabled              bool              `json:"enabled"`
	SortOrder            int               `json:"sortOrder"`
	CreatedAt            int64             `json:"createdAt"`
	UpdatedAt            int64             `json:"updatedAt"`
}

func placesToExport(rows []store.Place) []placeExport {
	out := make([]placeExport, 0, len(rows))
	for _, p := range rows {
		out = append(out, placeExport{
			ID:                   p.ID,
			Name:                 p.Name,
			Provider:             p.Provider,
			Kind:                 p.Kind,
			Base:                 p.Base,
			Folders:              p.Folders,
			CredsRef:             p.CredsRef,
			OffPremises:          p.OffPremises,
			StorageClass:         p.StorageClass,
			Immutable:            p.Immutable,
			RetentionKeepLast:    p.RetentionKeepLast,
			RetentionKeepDaily:   p.RetentionKeepDaily,
			RetentionKeepWeekly:  p.RetentionKeepWeekly,
			RetentionKeepMonthly: p.RetentionKeepMonthly,
			LimitUpload:          p.LimitUpload,
			LimitDownload:        p.LimitDownload,
			GrowthBudgetGB:       p.GrowthBudgetGB,
			Enabled:              p.Enabled,
			SortOrder:            p.SortOrder,
			CreatedAt:            p.CreatedAt,
			UpdatedAt:            p.UpdatedAt,
		})
	}
	return out
}
