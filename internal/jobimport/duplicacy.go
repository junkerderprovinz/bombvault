package jobimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// duplicacyPreference is one entry of .duplicacy/preferences, as the CLI's
// Preference type writes it.
type duplicacyPreference struct {
	Name               string            `json:"name"`
	SnapshotID         string            `json:"id"`
	RepositoryPath     string            `json:"repository"`
	StorageURL         string            `json:"storage"`
	Encrypted          bool              `json:"encrypted"`
	BackupProhibited   bool              `json:"no_backup"`
	NobackupFile       string            `json:"nobackup_file"`
	Keys               map[string]string `json:"keys"`
	FiltersFile        string            `json:"filters"`
	ExcludeByAttribute bool              `json:"exclude_by_attribute"`
}

// DuplicacyCLI reads a repository's preferences file. repo is the folder that
// holds .duplicacy, as the CLI saw it, and read reaches the files next to the
// preferences. Every storage the repository backs up to is a job of its own;
// one marked no_backup only restores and is left out.
func DuplicacyCLI(prefs []byte, repo string, read Reader) ([]Job, error) {
	var entries []duplicacyPreference
	if err := json.Unmarshal(prefs, &entries); err != nil {
		return nil, fmt.Errorf("not a Duplicacy preferences file: %w", err)
	}
	if len(entries) == 0 {
		return nil, errors.New("the Duplicacy preferences file names no storage")
	}
	var jobs []Job
	for _, p := range entries {
		if p.BackupProhibited {
			continue
		}
		source := slashPath(p.RepositoryPath)
		if source == "" {
			source = slashPath(repo)
		}
		job := Job{Tool: ToolDuplicacy, Name: p.SnapshotID, Enabled: true, Sources: []string{source}}
		if p.Name != "default" {
			job.Name = strings.Trim(p.SnapshotID+"-"+p.Name, "-")
		}
		filters := p.FiltersFile
		if filters == "" {
			filters = "filters"
		}
		if data, err := read(filters); err == nil {
			job.duplicacyFilters(read, filters, data, source, nil)
		} else if p.FiltersFile != "" {
			job.note(NoteFilter, "@"+p.FiltersFile)
		}
		if p.Encrypted {
			job.note(NoteEncryptionKey, "")
		}
		job.duplicacyOptions(p.NobackupFile, p.ExcludeByAttribute, "")
		job.note(NoteNoSchedule, "")

		var notes []Note
		job.Dest, notes = duplicacyDestination(p.StorageURL, func(kind string) string {
			if p.Name != "default" {
				if v := p.Keys[p.Name+"_"+kind]; v != "" {
					return v
				}
			}
			return p.Keys[kind]
		})
		job.Notes = append(job.Notes, notes...)
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// duplicacyWebConfig is the part of Duplicacy Web's duplicacy.json the import
// reads. Duplicacy Web publishes no description of the file; the names and
// types are those of the Storage, Computer, Repository, Schedule and Job
// types in the 1.8 build.
type duplicacyWebConfig struct {
	Storages  []duplicacyWebStorage `json:"storages"`
	Computers []struct {
		Name         string `json:"name"`
		Repositories []struct {
			Index              int    `json:"index"`
			ID                 string `json:"id"`
			Path               string `json:"path"`
			Storage            string `json:"storage"`
			NobackupFile       string `json:"nobackup_file"`
			ExcludeByAttribute bool   `json:"exclude_by_attribute"`
		} `json:"repositories"`
	} `json:"computers"`
	Schedules []struct {
		Name      string `json:"name"`
		Start     string `json:"start"`
		Frequency int    `json:"frequency"`
		Days      string `json:"days"`
		Jobs      []struct {
			Type       string `json:"type"`
			Computer   string `json:"computer"`
			Repository int    `json:"repository"`
			Storage    string `json:"storage"`
			Options    string `json:"options"`
		} `json:"jobs"`
	} `json:"schedules"`
}

type duplicacyWebStorage struct {
	Name      string `json:"name"`
	URL       string `json:"url"`
	Encrypted bool   `json:"encrypted"`
}

// DuplicacyWeb reads duplicacy.json. read reaches the files of the folder it
// sits in, where Duplicacy Web keeps the patterns of each backup under
// filters/<computer>/<index>. The sign-ins in the file are encrypted with
// the Duplicacy Web password, so none is carried over.
func DuplicacyWeb(config []byte, read Reader) ([]Job, error) {
	var c duplicacyWebConfig
	if err := json.Unmarshal(config, &c); err != nil {
		return nil, fmt.Errorf("not a Duplicacy Web configuration: %w", err)
	}
	if c.Storages == nil && c.Computers == nil {
		return nil, errors.New("not a Duplicacy Web configuration: it has no storages and no computers")
	}
	var jobs []Job
	for _, computer := range c.Computers {
		for _, r := range computer.Repositories {
			source := slashPath(r.Path)
			job := Job{Tool: ToolDuplicacyWeb, Name: r.ID, Enabled: true, Sources: []string{source}}
			filters := "filters/" + computer.Name + "/" + strconv.Itoa(r.Index)
			if data, err := read(filters); err == nil {
				job.duplicacyFilters(read, filters, data, source, nil)
			}

			scheduled, retention := false, false
			var options string
			for _, s := range c.Schedules {
				for _, sj := range s.Jobs {
					if sj.Computer != computer.Name || sj.Repository != r.Index {
						continue
					}
					switch {
					case sj.Type == "prune":
						retention = true
					case sj.Type == "backup" && !scheduled:
						scheduled, options = true, sj.Options
						if cadence, ok := duplicacyWebCadence(s.Start, s.Frequency, s.Days); ok {
							job.Schedule = cadence
						} else {
							job.note(NoteSchedule, s.Name)
						}
					}
				}
			}
			if !scheduled {
				job.note(NoteNoSchedule, "")
			}
			if retention {
				job.note(NoteRetention, "")
			}
			job.duplicacyOptions(r.NobackupFile, r.ExcludeByAttribute, options)

			i := slices.IndexFunc(c.Storages, func(s duplicacyWebStorage) bool {
				return s.Name == r.Storage
			})
			if i < 0 {
				job.note(NoteDestination, "")
				jobs = append(jobs, job)
				continue
			}
			storage := c.Storages[i]
			if storage.Encrypted {
				job.note(NoteEncryptionKey, "")
			}
			var notes []Note
			job.Dest, notes = duplicacyDestination(storage.URL, func(string) string { return "" })
			job.Notes = append(job.Notes, notes...)
			if job.Dest.Kind == KindS3 || job.Dest.Kind == KindB2 {
				job.note(NoteSignInEncrypted, "")
			}
			jobs = append(jobs, job)
		}
	}
	return jobs, nil
}

func (j *Job) duplicacyOptions(nobackupFile string, byAttribute bool, options string) {
	var left []string
	if options = strings.TrimSpace(options); options != "" {
		left = append(left, options)
	}
	if nobackupFile != "" {
		left = append(left, "nobackup_file "+nobackupFile)
	}
	if byAttribute {
		left = append(left, "exclude_by_attribute")
	}
	if len(left) > 0 {
		j.note(NoteOptions, strings.Join(left, ", "))
	}
}

// duplicacyFilters translates the lines of a Duplicacy filters file. Patterns
// are relative to the repository, a star matches slashes too, and the first
// pattern that matches decides, so an include rule has no counterpart in a
// list of restic excludes and is only noted. An @ line pulls in another file,
// relative to the one it stands in.
func (j *Job) duplicacyFilters(read Reader, file string, data []byte, repo string, seen []string) {
	seen = append(seen, file)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "", line[0] == '#', line == "+", line == "-", line == "i:", line == "e:":
			continue
		case line[0] == '@':
			inc := strings.TrimSpace(line[1:])
			if inc == "" {
				continue
			}
			if !path.IsAbs(inc) {
				inc = path.Join(path.Dir(file), inc)
			}
			incData, err := read(inc)
			if err != nil || slices.Contains(seen, inc) {
				j.note(NoteFilter, line)
				continue
			}
			j.duplicacyFilters(read, inc, incData, repo, seen)
		case strings.HasPrefix(line, "e:"):
			j.note(NoteFilter, line)
		case line[0] != '-':
			j.note(NoteInclude, line)
		default:
			p := strings.TrimPrefix(line[1:], "/")
			if !hasWildcard(p) {
				j.Excludes = append(j.Excludes, repo+"/"+strings.TrimSuffix(p, "/"))
			} else if name, ok := namePattern(p); ok {
				j.Excludes = append(j.Excludes, name)
			} else {
				j.note(NoteFilter, line)
			}
		}
	}
}

var duplicacyStart = regexp.MustCompile(`^([0-9][0-9]):([0-9][0-9])$`)

// duplicacyWebCadence turns a Duplicacy Web schedule into a BombVault
// cadence. start is HH:MM, frequency is in seconds, and days is a binary
// number whose lowest bit is Monday and whose highest is Sunday.
func duplicacyWebCadence(start string, frequency int, days string) (string, bool) {
	m := duplicacyStart.FindStringSubmatch(start)
	bits, err := strconv.ParseInt(days, 2, 32)
	if m == nil || err != nil || bits <= 0 || bits > 0x7f {
		return "", false
	}
	hour, _ := strconv.Atoi(m[1])
	minute, _ := strconv.Atoi(m[2])
	if hour > 23 || minute > 59 {
		return "", false
	}
	var on []time.Weekday
	for i := range 7 {
		if bits&(1<<i) != 0 {
			on = append(on, time.Weekday((i+1)%7))
		}
	}
	switch {
	case frequency == 86400 && len(on) == 7:
		return "daily " + start, true
	case frequency == 86400:
		return "weekly " + dayList(on) + " " + start, true
	case frequency > 0 && frequency < 86400 && frequency%3600 == 0 && 86400%frequency == 0:
		return everyHours(minute, hour, frequency/3600, cronDays(on)), true
	case frequency > 0 && frequency < 3600 && frequency%60 == 0 && 3600%frequency == 0:
		return everyMinutes(minute, frequency/60, cronDays(on)), true
	}
	return "", false
}

// duplicacyStorageURL is the pattern the CLI splits a storage URL with.
var duplicacyStorageURL = regexp.MustCompile(`^([\w-]+)://([\w\-@\.\!]+@)?([^/]+)(/(.+))?`)

// duplicacyDestination reads a storage URL the way the CLI's CreateStorage
// does. key returns a saved credential of the kind the CLI asks for, such as
// s3_id, or nothing.
func duplicacyDestination(raw string, key func(kind string) string) (Destination, []Note) {
	raw = strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(raw, "/"), hasDrive(raw), strings.HasPrefix(raw, `\\`):
		return Destination{Kind: KindLocal, Path: slashPath(raw)}, nil
	case strings.HasPrefix(raw, "flat://"):
		return Destination{Kind: KindLocal, Path: slashPath(raw[len("flat://"):])}, nil
	case strings.HasPrefix(raw, "samba://"):
		return Destination{Kind: KindLocal, Path: slashPath(raw[len("samba://"):])}, nil
	}
	m := duplicacyStorageURL.FindStringSubmatch(raw)
	if m == nil {
		return Destination{}, []Note{{Code: NoteDestination}}
	}
	scheme, user, host, dir := m[1], m[2], m[3], m[5]
	region := strings.TrimSuffix(user, "@")
	// The CLI's pattern leaves a user with a password in the host part.
	host, infoUser, password := splitUserinfo(host)
	s3 := func(proto, idKind, secretKind string) Destination {
		if strings.EqualFold(host, "amazon") || strings.EqualFold(host, "amazon.com") {
			host = "s3.amazonaws.com"
		}
		id, secret := signIn(key(idKind), key(secretKind), infoUser, password)
		return Destination{Kind: KindS3, Location: "s3:" + proto + "://" + underSubfolder(host, dir), Region: region, KeyID: id, Secret: secret}
	}
	switch scheme {
	case "s3", "s3c", "minios":
		return s3("https", "s3_id", "s3_secret"), nil
	case "minio":
		return s3("http", "s3_id", "s3_secret"), nil
	case "wasabi":
		return s3("https", "wasabi_key", "wasabi_secret"), nil
	case "b2":
		id, secret := signIn(key("b2_id"), key("b2_key"), infoUser, password)
		return Destination{Kind: KindB2, Path: underSubfolder(host, dir), KeyID: id, Secret: secret}, nil
	case "sftp", "sftpc":
		if user == "" && infoUser != "" {
			user = infoUser + "@"
		}
		return sftpDestination(user+host, dir), []Note{{Code: NoteSSHKey}}
	}
	return Destination{}, []Note{{Code: NoteDestination, Detail: scheme}}
}
