package dbdump

import (
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ResultLinePrefix marks the single line the helper writes to stderr to say how
// the dump went. restic forwards it prefixed with "subprocess <argv0>: ".
const ResultLinePrefix = "bombvault-dbdump-result "

// Reason ids are the helper's vocabulary. The api maps each of them to exactly
// one stored run reason.
const (
	ReasonAuth          = "auth"
	ReasonPrivileges    = "privileges"
	ReasonUnreachable   = "unreachable"
	ReasonNoClient      = "no-client"
	ReasonSecret        = "secret"
	ReasonNoCredentials = "no-credentials"
	ReasonNotRunning    = "not-running"
	ReasonTimeout       = "timeout"
	ReasonEmpty         = "empty"
	ReasonIncomplete    = "incomplete"
	ReasonTool          = "tool"
	ReasonDocker        = "docker"
	ReasonWrite         = "write"
	ReasonCancelled     = "cancelled"
	ReasonUsage         = "usage"
	ReasonNeedsUpgrade  = "needs-upgrade"
)

// ReasonIDs is every reason id, so a caller can prove its mapping is total.
var ReasonIDs = []string{
	ReasonAuth, ReasonPrivileges, ReasonUnreachable, ReasonNoClient,
	ReasonSecret, ReasonNoCredentials, ReasonNotRunning, ReasonTimeout,
	ReasonEmpty, ReasonIncomplete, ReasonTool, ReasonDocker, ReasonWrite,
	ReasonCancelled, ReasonUsage, ReasonNeedsUpgrade,
}

// maxDetail is the room a scrubbed tool message gets on the wire and in the
// stored run reason.
const maxDetail = 300

// Result is what the helper reports about one dump.
type Result struct {
	V             int    `json:"v"`
	OK            bool   `json:"ok"`
	Reason        string `json:"reason,omitempty"`
	Exit          int    `json:"exit"`
	Bytes         int64  `json:"bytes"`
	Scope         string `json:"scope,omitempty"`
	Detail        string `json:"detail,omitempty"`
	OrphanStopped *bool  `json:"orphanStopped,omitempty"`
}

// Line renders the result as the one line the helper writes to stderr.
func (r Result) Line() string {
	encoded, _ := json.Marshal(r)
	return ResultLinePrefix + string(encoded)
}

// ParseResult reads the last result line the helper wrote. A restic run can
// finish without one, which is not a failure by itself, so the caller decides
// what a missing result means.
func ParseResult(lines []string) (Result, bool) {
	for i := len(lines) - 1; i >= 0; i-- {
		at := strings.Index(lines[i], ResultLinePrefix)
		if at < 0 {
			continue
		}
		var r Result
		if err := json.Unmarshal([]byte(lines[i][at+len(ResultLinePrefix):]), &r); err != nil {
			continue
		}
		if r.V != 1 || r.Bytes < 0 || len(r.Detail) > maxDetail {
			continue
		}
		if r.Reason != "" && !knownReason(r.Reason) {
			continue
		}
		return r, true
	}
	return Result{}, false
}

func knownReason(id string) bool {
	for _, known := range ReasonIDs {
		if id == known {
			return true
		}
	}
	return false
}

// ParsePID reads the pid of the dump process from the last pid line.
func ParsePID(lines []string) (int, bool) {
	for i := len(lines) - 1; i >= 0; i-- {
		at := strings.Index(lines[i], PIDLinePrefix)
		if at < 0 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(lines[i][at+len(PIDLinePrefix):]))
		if err != nil || pid <= 1 {
			continue
		}
		return pid, true
	}
	return 0, false
}

// ParseScope reads what the dump covered from the last scope line.
func ParseScope(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		at := strings.Index(lines[i], ScopeLinePrefix)
		if at < 0 {
			continue
		}
		if scope := strings.TrimSpace(lines[i][at+len(ScopeLinePrefix):]); scope != "" {
			return scope
		}
	}
	return ""
}

var secretMarkers = []string{"password=", "pwd=", "pass:"}

// ScrubDetail makes a tool's message fit to store: one line, no control
// characters, no credential left in it, at most maxDetail bytes.
func ScrubDetail(s string) string {
	oneLine := strings.Join(strings.Fields(stripControl(s)), " ")
	return capDetail(redactSecrets(oneLine))
}

func stripControl(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, s)
}

func redactSecrets(s string) string {
	lower := strings.ToLower(s)
	var out strings.Builder
	for i := 0; i < len(s); {
		marker := ""
		for _, m := range secretMarkers {
			if strings.HasPrefix(lower[i:], m) {
				marker = m
				break
			}
		}
		if marker == "" {
			out.WriteByte(s[i])
			i++
			continue
		}
		out.WriteString(s[i : i+len(marker)])
		i += len(marker)
		for i < len(s) && s[i] != ' ' {
			i++
		}
		out.WriteString("***")
	}
	return out.String()
}

func capDetail(s string) string {
	if len(s) <= maxDetail {
		return s
	}
	cut := maxDetail
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
