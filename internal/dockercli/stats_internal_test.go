package dockercli

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

// Trimmed from a one-shot stats reply of Docker 29.5 on Unraid.
const bridgeStats = `{"read":"2026-09-27T18:39:35.142327335Z","cpu_stats":{"cpu_usage":{"total_usage":17665000}},
"networks":{"eth0":{"rx_bytes":1046,"tx_bytes":126},"eth1":{"rx_bytes":4,"tx_bytes":10}},"pids_stats":{"current":1}}`

const hostNetStats = `{"read":"2026-09-27T18:39:35Z","cpu_stats":{"cpu_usage":{"total_usage":5}},"pids_stats":{"current":3}}`

const stoppedStats = `{"read":"0001-01-01T00:00:00Z","cpu_stats":{"cpu_usage":{"total_usage":0}},"pids_stats":{}}`

func decodeStats(t *testing.T, raw string) container.StatsResponse {
	t.Helper()
	var st container.StatsResponse
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestStatsSampleSumsEveryNetwork(t *testing.T) {
	s := statsSample(decodeStats(t, bridgeStats), time.Unix(1, 0))
	if !s.Running || !s.HasNet || s.RxBytes != 1050 || s.TxBytes != 136 || s.CPUNanos != 17665000 {
		t.Fatalf("sample = %+v", s)
	}
}

func TestHostNetworkedContainerHasNoNetworkCounters(t *testing.T) {
	s := statsSample(decodeStats(t, hostNetStats), time.Unix(1, 0))
	if !s.Running || s.HasNet {
		t.Fatalf("sample = %+v, want running without network counters", s)
	}
}

func TestStoppedContainerIsNotRunning(t *testing.T) {
	if s := statsSample(decodeStats(t, stoppedStats), time.Unix(1, 0)); s.Running {
		t.Fatalf("sample = %+v, want not running", s)
	}
}
