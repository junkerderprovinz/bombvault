package zfs

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Properties are the locally set properties of one dataset, keyed by name,
// with values in the parseable form zfs get -p prints.
type Properties map[string]string

// createOnlyDefaults are the properties that can only be set when a dataset is
// created. zfs get reports them with no source at all, so a changed one is
// recognised by differing from the default.
var createOnlyDefaults = map[string]string{
	"casesensitivity": "sensitive",
	"normalization":   "none",
	"utf8only":        "off",
}

// notApplied are stored and shown but never set by BombVault: the mountpoint
// would collide with the original dataset, canmount and readonly would stop
// the restore from writing, and the key properties need a key BombVault does
// not have.
var notApplied = map[string]bool{
	"mountpoint":  true,
	"canmount":    true,
	"readonly":    true,
	"encryption":  true,
	"keyformat":   true,
	"keylocation": true,
	"pbkdf2iters": true,
}

// CreateOnly reports whether a property can only be passed to zfs create.
func CreateOnly(name string) bool {
	_, ok := createOnlyDefaults[name]
	return ok
}

// NotApplied reports whether BombVault leaves a stored property alone.
func NotApplied(name string) bool { return notApplied[name] }

var (
	propNameRe = regexp.MustCompile(`^[a-z0-9_.:-]+$`)
	// propSources are what PropertiesArgs asks for: local for everything set
	// on the dataset itself, none for the creation-time properties.
	propSources = "local,none"
)

// PropertiesArgs reads the locally set and the creation-time properties of
// every filesystem in a tree.
func PropertiesArgs(root string) ([]string, error) {
	if err := validateNameChars(root); err != nil {
		return nil, err
	}
	return []string{zfsBinary, "get", "-H", "-p", "-r", "-t", "filesystem", "-s", propSources, "-o", "name,property,value,source", "all", root}, nil
}

// ParseProperties keeps, per dataset, what was set locally plus the
// creation-time properties that differ from their default. Read-only and
// inherited values are left out: they come back on their own.
func ParseProperties(out string) (map[string]Properties, error) {
	res := map[string]Properties{}
	for _, line := range splitLines(out) {
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("zfs get: %d fields, want 4", len(f))
		}
		name, prop, value, source := f[0], f[1], f[2], f[3]
		keep := source == "local"
		if def, ok := createOnlyDefaults[prop]; ok && source == "-" {
			keep = value != def && value != "-"
		}
		if !keep {
			continue
		}
		if res[name] == nil {
			res[name] = Properties{}
		}
		res[name][prop] = value
	}
	return res, nil
}

// assignments renders the properties for an argv in a stable order, leaving
// out the ones skip names. A name or value that could not have come from zfs
// get is refused rather than passed on.
func assignments(p Properties, skip func(string) bool) ([]string, error) {
	keys := make([]string, 0, len(p))
	for k := range p {
		if !skip(k) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := p[k]
		if !propNameRe.MatchString(k) || strings.ContainsAny(v, "\x00\n") {
			return nil, &NameError{Code: "invalid-name", Reason: "not a property BombVault can pass on: " + k}
		}
		out = append(out, k+"="+v)
	}
	return out, nil
}

// CreateArgs creates a dataset with the stored properties it can take at
// creation. The parent has to exist; nothing above it is created.
func CreateArgs(dataset string, p Properties) ([]string, error) {
	if err := ValidateDatasetName(dataset); err != nil {
		return nil, err
	}
	set, err := assignments(p, NotApplied)
	if err != nil {
		return nil, err
	}
	args := []string{zfsBinary, "create"}
	for _, a := range set {
		args = append(args, "-o", a)
	}
	return append(args, dataset), nil
}

// SetArgs applies the stored properties an existing dataset can still take.
// It returns nil when there is none.
func SetArgs(dataset string, p Properties) ([]string, error) {
	if err := ValidateDatasetName(dataset); err != nil {
		return nil, err
	}
	set, err := assignments(p, func(k string) bool { return NotApplied(k) || CreateOnly(k) })
	if err != nil || len(set) == 0 {
		return nil, err
	}
	args := append([]string{zfsBinary, "set"}, set...)
	return append(args, dataset), nil
}
