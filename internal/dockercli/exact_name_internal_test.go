package dockercli

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
)

// nextcloudWithDbID is a running container whose id starts with "db", which
// Docker hands out for a name it does not know.
func nextcloudWithDbID() *container.InspectResponse {
	return running("db2f00d"+strings.Repeat("0", 57), "nextcloud", container.HostConfig{})
}

func TestARemovedNameThatLooksLikeAnIDPrefixIsNotFound(t *testing.T) {
	_, c := newEngine(t, nextcloudWithDbID())
	ctx := context.Background()

	if _, err := c.Inspect(ctx, "db"); !IsNotFound(err) {
		t.Errorf("Inspect = %v, want not found", err)
	}
	if name, err := c.InspectName(ctx, "db"); name != "" || err != nil {
		t.Errorf("InspectName = %q, %v, want no container", name, err)
	}
	if id, err := c.ContainerID(ctx, "db"); id != "" || err != nil {
		t.Errorf("ContainerID = %q, %v, want no container", id, err)
	}
}

func TestARemovedNameThatLooksLikeAnIDPrefixTouchesNoOtherContainer(t *testing.T) {
	e, c := newEngine(t, nextcloudWithDbID())
	ctx := context.Background()

	if err := c.Stop(ctx, "db", time.Second); !IsNotFound(err) {
		t.Errorf("Stop = %v, want not found", err)
	}
	if err := c.Start(ctx, "db"); !IsNotFound(err) {
		t.Errorf("Start = %v, want not found", err)
	}
	if err := c.Restart(ctx, "db", time.Second); !IsNotFound(err) {
		t.Errorf("Restart = %v, want not found", err)
	}
	if err := c.Signal(ctx, "db", "SIGHUP"); !IsNotFound(err) {
		t.Errorf("Signal = %v, want not found", err)
	}
	if err := c.Remove(ctx, "db"); err != nil {
		t.Errorf("Remove = %v, want nothing to remove", err)
	}
	if slices.ContainsFunc(e.calls, func(call string) bool { return !strings.HasPrefix(call, "GET") && !strings.HasPrefix(call, "inspect") }) {
		t.Errorf("Docker was told to change something: %v", e.calls)
	}
}

func TestAContainerIsStillFoundByItsNameAndByItsID(t *testing.T) {
	c0 := nextcloudWithDbID()
	_, c := newEngine(t, c0)
	ctx := context.Background()

	for _, ref := range []string{"nextcloud", "/nextcloud", c0.ID, c0.ID[:12]} {
		if in, err := c.Inspect(ctx, ref); err != nil || in.Name != "/nextcloud" {
			t.Errorf("Inspect(%q) = %q, %v", ref, in.Name, err)
		}
	}
}
