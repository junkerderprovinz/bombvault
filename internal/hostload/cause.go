package hostload

// Kinds of cause.
const (
	CauseDisk     = "disk"
	CauseCPU      = "cpu"
	CauseCPULimit = "cpulimit"
	CauseUpload   = "upload"
)

// Cause is the one resource that held a run back. Share is how busy it was,
// or for an upload the share of the repository's upload limit it used.
type Cause struct {
	Kind  string  `json:"kind"`
	Name  string  `json:"name,omitempty"`
	Role  string  `json:"role,omitempty"`
	Share float64 `json:"share"`
}

// saturated is the share from which a resource counts as the brake.
const saturated = 0.9

// CauseOf names the one resource that was saturated over the run, and
// nothing when none was or several were, since then the answer is a guess.
// uploadLimitBps is the repository's upload limit, 0 for none.
func CauseOf(s Summary, uploadLimitBps float64) *Cause {
	var found []Cause
	for _, d := range s.Disks {
		if d.Busy >= saturated {
			name := d.Label
			if name == "" {
				name = d.Name
			}
			found = append(found, Cause{Kind: CauseDisk, Name: name, Role: d.Role, Share: d.Busy})
		}
	}
	if s.CPU != nil && *s.CPU >= saturated {
		found = append(found, Cause{Kind: CauseCPU, Share: *s.CPU})
	}
	if s.CPULimit != nil && *s.CPULimit >= saturated {
		found = append(found, Cause{Kind: CauseCPULimit, Share: *s.CPULimit})
	}
	if uploadLimitBps > 0 && s.UploadBps != nil && *s.UploadBps >= saturated*uploadLimitBps {
		found = append(found, Cause{Kind: CauseUpload, Share: min(*s.UploadBps/uploadLimitBps, 1)})
	}
	if len(found) != 1 {
		return nil
	}
	return &found[0]
}
