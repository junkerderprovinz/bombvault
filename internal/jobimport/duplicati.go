package jobimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// ErrEncryptedExport refuses an export Duplicati wrote with a password. Its
// content is AES Crypt, which BombVault does not open.
var ErrEncryptedExport = errors.New("the export is encrypted: export the job again without a password")

// duplicatiExport is the file Duplicati writes when a job is exported, its
// ImportExportStructure serialized by Newtonsoft.Json.
type duplicatiExport struct {
	Schedule *duplicatiSchedule `json:"Schedule"`
	Backup   *struct {
		Name      string   `json:"Name"`
		TargetURL string   `json:"TargetURL"`
		Enabled   *bool    `json:"Enabled"`
		Sources   []string `json:"Sources"`
		Settings  []struct {
			Name  string `json:"Name"`
			Value string `json:"Value"`
		} `json:"Settings"`
		Filters []duplicatiFilter `json:"Filters"`
	} `json:"Backup"`
}

type duplicatiSchedule struct {
	Time        string   `json:"Time"`
	Repeat      string   `json:"Repeat"`
	Rule        string   `json:"Rule"`
	AllowedDays []string `json:"AllowedDays"`
}

type duplicatiFilter struct {
	Order      int64  `json:"Order"`
	Include    bool   `json:"Include"`
	Expression string `json:"Expression"`
}

// duplicatiQuiet are settings that need no note: BombVault compresses,
// encrypts and sizes its packs on its own.
var duplicatiQuiet = map[string]bool{
	"encryption-module":  true,
	"compression-module": true,
	"dblock-size":        true,
	"no-encryption":      true,
}

var duplicatiRetention = map[string]bool{"keep-time": true, "keep-versions": true, "retention-policy": true}

// Duplicati reads one exported job. loc is the server's time zone, which the
// schedule is written in; Duplicati keeps its times in UTC.
func Duplicati(data []byte, loc *time.Location) (Job, error) {
	if bytes.HasPrefix(data, []byte("AES")) {
		return Job{}, ErrEncryptedExport
	}
	var ex duplicatiExport
	if err := json.Unmarshal(data, &ex); err != nil {
		return Job{}, fmt.Errorf("not a Duplicati export: %w", err)
	}
	if ex.Backup == nil {
		return Job{}, errors.New("not a Duplicati export: it holds no backup job")
	}
	b := ex.Backup
	job := Job{Tool: ToolDuplicati, Name: strings.TrimSpace(b.Name), Enabled: b.Enabled == nil || *b.Enabled}
	if !job.Enabled {
		job.note(NoteDisabled, "")
	}
	for _, s := range b.Sources {
		job.Sources = append(job.Sources, slashPath(s))
	}

	var options []string
	retention := false
	for _, s := range b.Settings {
		name := strings.TrimPrefix(s.Name, "--")
		switch {
		case name == "passphrase":
			if s.Value != "" {
				job.note(NoteEncryptionKey, "")
			}
		case duplicatiRetention[name]:
			if s.Value != "" && !retention {
				retention = true
				job.note(NoteRetention, "")
			}
		case !duplicatiQuiet[name]:
			options = append(options, s.Name)
		}
	}
	if len(options) > 0 {
		job.note(NoteOptions, strings.Join(options, ", "))
	}

	filters := slices.Clone(b.Filters)
	slices.SortStableFunc(filters, func(a, b duplicatiFilter) int { return int(a.Order - b.Order) })
	for _, f := range filters {
		job.duplicatiFilter(f)
	}

	job.duplicatiSchedule(ex.Schedule, loc)
	var notes []Note
	job.Dest, notes = duplicatiDestination(b.TargetURL)
	job.Notes = append(job.Notes, notes...)
	return job, nil
}

func (j *Job) duplicatiFilter(f duplicatiFilter) {
	expr := strings.TrimSpace(f.Expression)
	switch {
	case expr == "":
		return
	case f.Include:
		j.note(NoteInclude, expr)
		return
	case strings.HasPrefix(expr, "[") && strings.HasSuffix(expr, "]"),
		strings.HasPrefix(expr, "{") && strings.HasSuffix(expr, "}"):
		j.note(NoteFilter, expr)
		return
	}
	p := slashPath(expr)
	if IsAnchored(p) && !hasWildcard(p) {
		j.Excludes = append(j.Excludes, p)
		return
	}
	if name, ok := namePattern(p); ok {
		j.Excludes = append(j.Excludes, name)
		return
	}
	j.note(NoteFilter, expr)
}

func (j *Job) duplicatiSchedule(s *duplicatiSchedule, loc *time.Location) {
	if s == nil || strings.TrimSpace(s.Repeat) == "" {
		j.note(NoteNoSchedule, "")
		return
	}
	at, err := duplicatiTime(s.Time)
	if err != nil {
		j.note(NoteSchedule, s.Repeat)
		return
	}
	cadence, ok := duplicatiCadence(s.Repeat, at.In(loc), duplicatiDays(s))
	if !ok {
		j.note(NoteSchedule, s.Repeat)
		return
	}
	j.Schedule = cadence
}

// duplicatiTime reads a DateTime as Newtonsoft writes it. One without a zone
// is UTC, which is how Duplicati stores its schedule.
func duplicatiTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.ParseInLocation("2006-01-02T15:04:05.999999999", s, time.UTC)
}

var weekdayNames = map[string]time.Weekday{
	"sun": time.Sunday, "mon": time.Monday, "tue": time.Tuesday, "wed": time.Wednesday,
	"thu": time.Thursday, "fri": time.Friday, "sat": time.Saturday,
}

// duplicatiDays reads the allowed days from the rule, which is what Duplicati
// stores, and from the list it derives, in case only that one is filled in.
func duplicatiDays(s *duplicatiSchedule) []time.Weekday {
	names := s.AllowedDays
	for _, part := range strings.Split(s.Rule, ";") {
		if list, ok := cutPrefixFold(part, "AllowedWeekDays="); ok {
			names = strings.Split(list, ",")
		}
	}
	var days []time.Weekday
	for _, n := range names {
		n = strings.ToLower(strings.TrimSpace(n))
		if len(n) < 3 {
			continue
		}
		if d, ok := weekdayNames[n[:3]]; ok && !slices.Contains(days, d) {
			days = append(days, d)
		}
	}
	return days
}

func cutPrefixFold(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return "", false
	}
	return s[len(prefix):], true
}

var duplicatiRepeat = regexp.MustCompile(`^(\d+)([smhDWMY])$`)

// duplicatiCadence turns a repeat such as "1D" or "6h", counted from at, into
// a BombVault cadence. A repeat that no cadence can express, such as every
// two days, reports false.
func duplicatiCadence(repeat string, at time.Time, days []time.Weekday) (string, bool) {
	m := duplicatiRepeat.FindStringSubmatch(strings.TrimSpace(repeat))
	if m == nil {
		return "", false
	}
	n, _ := strconv.Atoi(m[1])
	hhmm := at.Format("15:04")
	allDays := len(days) == 0 || len(days) == 7
	switch {
	case n == 1 && m[2] == "D":
		if allDays {
			return "daily " + hhmm, true
		}
		return "weekly " + dayList(days) + " " + hhmm, true
	case n == 1 && m[2] == "W":
		return "weekly " + dayList([]time.Weekday{at.Weekday()}) + " " + hhmm, true
	case m[2] == "h" && n > 0 && 24%n == 0:
		return everyHours(at.Minute(), at.Hour(), n, cronDays(days)), true
	case m[2] == "m" && n > 0 && 60%n == 0 && n < 60:
		return everyMinutes(at.Minute(), n, cronDays(days)), true
	case n == 1 && m[2] == "M":
		return fmt.Sprintf("%d %d %d * *", at.Minute(), at.Hour(), at.Day()), true
	case n == 1 && m[2] == "Y":
		return fmt.Sprintf("%d %d %d %d *", at.Minute(), at.Hour(), at.Day(), int(at.Month())), true
	}
	return "", false
}

// everyHours is a cron entry every n hours, on the hours that a start at
// hour:minute lands on.
func everyHours(minute, hour, n int, dow string) string {
	if n == 1 {
		return fmt.Sprintf("%d * * * %s", minute, dow)
	}
	return fmt.Sprintf("%d %d/%d * * %s", minute, hour%n, n, dow)
}

func everyMinutes(minute, n int, dow string) string {
	return fmt.Sprintf("%d/%d * * * %s", minute%n, n, dow)
}

// dayList names days the way a weekly cadence takes them, Sunday first.
func dayList(days []time.Weekday) string {
	days = slices.Sorted(slices.Values(days))
	names := make([]string, len(days))
	for i, d := range days {
		names[i] = d.String()[:3]
	}
	return strings.Join(names, ",")
}

func cronDays(days []time.Weekday) string {
	if len(days) == 0 || len(days) == 7 {
		return "*"
	}
	days = slices.Sorted(slices.Values(days))
	nums := make([]string, len(days))
	for i, d := range days {
		nums[i] = strconv.Itoa(int(d))
	}
	return strings.Join(nums, ",")
}

// duplicatiDestination reads a target URL as Duplicati's backends take it:
// scheme://server/path?options, with the sign-in among the options or, as
// Duplicati also takes it, in front of the server.
func duplicatiDestination(target string) (Destination, []Note) {
	scheme, rest, ok := strings.Cut(strings.TrimSpace(target), "://")
	if !ok {
		return Destination{}, []Note{{Code: NoteDestination}}
	}
	rest, rawQuery, _ := strings.Cut(rest, "?")
	if scheme == "file" {
		return Destination{Kind: KindLocal, Path: slashPath(rest)}, nil
	}
	opts, _ := url.ParseQuery(rawQuery)
	opt := func(names ...string) string {
		for _, n := range names {
			if v := opts.Get(n); v != "" {
				return v
			}
		}
		return ""
	}
	host, folder, _ := strings.Cut(rest, "/")
	folder = strings.TrimRight(folder, "/")
	host, user, password := splitUserinfo(host)

	switch scheme {
	case "s3":
		region := opt("s3-location-constraint")
		server := opt("s3-server-name")
		if server == "" {
			server = "s3.amazonaws.com"
			if region != "" {
				server = "s3." + region + ".amazonaws.com"
			}
		}
		proto := "http"
		if strings.EqualFold(opt("use-ssl"), "true") || strings.HasSuffix(server, ".amazonaws.com") {
			proto = "https"
		}
		id, secret := signIn(opt("auth-username", "aws-access-key-id"), opt("auth-password", "aws-secret-access-key"), user, password)
		return Destination{
			Kind:         KindS3,
			Location:     "s3:" + proto + "://" + underSubfolder(server, host, folder),
			Region:       region,
			StorageClass: opt("s3-storage-class"),
			KeyID:        id,
			Secret:       secret,
		}, nil
	case "b2":
		id, secret := signIn(opt("auth-username", "b2-accountid"), opt("auth-password", "b2-applicationkey"), user, password)
		return Destination{Kind: KindB2, Path: underSubfolder(host, folder), KeyID: id, Secret: secret}, nil
	case "ssh":
		if u := opt("auth-username"); u != "" {
			user = u
		}
		if user != "" {
			host = user + "@" + host
		}
		return sftpDestination(host, folder), []Note{{Code: NoteSSHKey}}
	case "rclone":
		return Destination{Kind: KindRclone, Location: "rclone:" + host + ":" + underSubfolder(folder)}, []Note{{Code: NoteRcloneRemote, Detail: host}}
	}
	return Destination{}, []Note{{Code: NoteDestination, Detail: scheme}}
}
