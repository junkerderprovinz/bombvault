package model

// ResourceNamePattern is the anchored regexp a Docker container or libvirt VM
// name must match: it starts with an alphanumeric and holds only
// [A-Za-z0-9._-], which leaves no room for a path separator or a leading "-"
// that argv would read as an option. Callers that reach a file sink also
// reject "..", which the character class alone still allows.
const ResourceNamePattern = `^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`
