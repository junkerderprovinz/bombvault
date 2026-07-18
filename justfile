# BombVault task runner — run `just` to list recipes.
# Recipes use sh (Git Bash on Windows). See CLAUDE.md for the full guide.

# List available recipes
default:
    @just --list

# Compile everything
build:
    go build ./...

# Run the full test suite (needs restic >= 0.17 on PATH)
test:
    go test ./...

# Format all Go code in place
fmt:
    gofmt -w .

# Fast pre-push chain: format check + vet + lint + tests + Dockerfile lint
check:
    gofmt -l . | (! grep .) || (echo "gofmt: run 'just fmt'"; exit 1)
    go vet ./...
    golangci-lint run ./...
    go test ./...
    hadolint Dockerfile

# Build the frontend SPA (embedded into the binary) — run when web/ changed
web:
    cd web && npm ci && npm run build

# Secret-scan the working tree
secrets:
    gitleaks dir . --redact --no-banner

# Scaffold release notes for a version, e.g. `just notes 6.3.0`
notes version:
    printf '## v{{version}}\n\n### Fixed\n\n### Changed\n' > .github/release-notes/v{{version}}.md
    cp .github/release-notes/v{{version}}.md internal/releasenotes/notes/v{{version}}.md
    @echo "Wrote both release-note copies for v{{version}} — edit, then commit."
