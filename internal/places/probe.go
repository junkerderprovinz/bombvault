package places

// FolderState is what a probe found at one folder of a place.
type FolderState string

const (
	// FolderEmpty is reachable and holds no repository yet.
	FolderEmpty FolderState = "empty"
	// FolderAbsent is a local folder the first backup creates.
	FolderAbsent FolderState = "absent"
	// FolderRepository holds a restic repository this instance opens.
	FolderRepository FolderState = "repository"
	// FolderFailed could not be probed; ProbeResult.Errors says why.
	FolderFailed FolderState = "error"
)

// ProbeError is why a folder could not be probed: the refusal code the
// interface translates, and the message.
type ProbeError struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"error"`
}

// ProbeFact is one thing a probe found out, as a translation key and its
// parameters.
type ProbeFact struct {
	Key    string            `json:"key"`
	Params map[string]string `json:"params,omitempty"`
}

// ProbeResult is what a connection test found, shaped as the answer of POST
// /api/places/probe: OK, Code and Error are the envelope's own fields. Fields
// is the form as the probe completed it, without secrets, for the add request
// to send back. RepoIDs and Errors are keyed by domain.
type ProbeResult struct {
	OK      bool                   `json:"ok"`
	Code    string                 `json:"code,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Base    string                 `json:"base,omitempty"`
	Fields  map[string]string      `json:"fields,omitempty"`
	Buckets []string               `json:"buckets,omitempty"`
	Facts   []ProbeFact            `json:"facts,omitempty"`
	Folders map[string]FolderState `json:"folders,omitempty"`
	RepoIDs map[string]string      `json:"repoIds,omitempty"`
	Errors  map[string]ProbeError  `json:"errors,omitempty"`
}

// FactBaseIsRepository says the address of a new place holds a repository
// already, so the place is offered as that repository.
const FactBaseIsRepository = "places.probe.baseIsRepository"

const (
	// FactBucketsHidden says the key may not list buckets, so the bucket is typed.
	FactBucketsHidden = "places.probe.bucketsHidden"
	// FactBucketNew names a typed bucket the key does not see; restic creates it
	// on the first backup. Params: bucket.
	FactBucketNew = "places.probe.bucketNew"
)

const (
	// FactB2Bucket names the one bucket a B2 key is limited to. Params: bucket.
	FactB2Bucket = "places.probe.b2Bucket"
	// FactB2Prefix names the folder a B2 key is limited to. Params: prefix.
	FactB2Prefix = "places.probe.b2Prefix"
)
