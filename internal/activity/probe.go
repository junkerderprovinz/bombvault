package activity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Ways a probe reached the server.
const (
	ViaDirect = "direct"
	ViaExec   = "exec"
)

// directTimeout bounds the request BombVault sends itself. A container on a
// macvlan network usually cannot be reached from BombVault's bridge at all, and
// the exec that follows should not wait long for that answer.
const directTimeout = 5 * time.Second

// maxAnswer caps how much of a session list is read.
const maxAnswer = 4 << 20

// Exec runs argv inside the container and returns its stdout and exit code.
type Exec func(ctx context.Context, argv []string) (string, int, error)

// Probe asks one media server for its sessions.
type Probe struct {
	Preset Preset
	Token  string
	// Addr is the container's address as BombVault sees it, "" when there is
	// none, as for a container on the host network.
	Addr string
	// Client sends the direct request; nil uses a client with directTimeout.
	Client *http.Client
	// Exec is the way in through the container, with its own curl or wget.
	Exec Exec
}

// Fetch reads the session list. It asks the server directly when the
// container has an address and goes through the container when that fails or
// there is none. via names the way that answered.
func (p Probe) Fetch(ctx context.Context) (body []byte, via string, err error) {
	var direct error
	if p.Addr != "" {
		body, direct = p.fetchDirect(ctx)
		if direct == nil {
			return body, ViaDirect, nil
		}
	}
	if p.Exec == nil {
		return nil, "", direct
	}
	body, err = p.fetchExec(ctx)
	if err != nil {
		if direct != nil {
			return nil, "", fmt.Errorf("%w; through the container: %w", direct, err)
		}
		return nil, "", err
	}
	return body, ViaExec, nil
}

func (p Probe) url(host string) string {
	return "http://" + net.JoinHostPort(host, strconv.Itoa(p.Preset.Port())) + p.Preset.Path()
}

func (p Probe) fetchDirect(ctx context.Context) ([]byte, error) {
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: directTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url(p.Addr), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", p.accept())
	if p.Token != "" {
		req.Header.Set(p.Preset.Header(p.Token))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ask %s at %s: %w", p.Preset.Name(), p.Addr, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAnswer))
	if err != nil {
		return nil, fmt.Errorf("read the answer of %s: %w", p.Preset.Name(), err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s at %s answered %s", p.Preset.Name(), p.Addr, resp.Status)
	}
	return body, nil
}

func (p Probe) accept() string {
	if p.Preset == Plex {
		return "application/xml"
	}
	return "application/json"
}

// execScript fetches $1 with the header $2: $3 through whichever of curl and
// wget the image has. The token travels as an argument so no shell quoting can
// break it.
const execScript = `if command -v curl >/dev/null 2>&1; then exec curl -fsS -m 10 -H "Accept: $4" -H "$2: $3" "$1"; ` +
	`elif command -v wget >/dev/null 2>&1; then exec wget -q -T 10 -O - --header="Accept: $4" --header="$2: $3" "$1"; ` +
	`else echo "the container has neither curl nor wget" >&2; exit 127; fi`

// ExecArgv is the command the way through the container runs.
func (p Probe) ExecArgv() []string {
	name, value := p.Preset.Header(p.Token)
	if p.Token == "" {
		// The script always sends a second header; without a token it repeats
		// the first.
		name, value = "Accept", p.accept()
	}
	return []string{"sh", "-c", execScript, "sh", p.url("127.0.0.1"), name, value, p.accept()}
}

func (p Probe) fetchExec(ctx context.Context) ([]byte, error) {
	out, code, err := p.Exec(ctx, p.ExecArgv())
	if err != nil {
		return nil, fmt.Errorf("ask %s through the container: %w", p.Preset.Name(), err)
	}
	switch code {
	case 0:
		return []byte(out), nil
	case 127:
		return nil, errors.New("the container has neither curl nor wget to ask " + p.Preset.Name())
	}
	return nil, fmt.Errorf("asking %s through the container ended with exit code %d%s", p.Preset.Name(), code, firstLine(out))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	line, _, _ := strings.Cut(s, "\n")
	if len(line) > 120 {
		line = line[:120]
	}
	return ": " + line
}
