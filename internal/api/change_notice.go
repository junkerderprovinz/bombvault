package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

// changeUpdated marks an image pulled again under the same name.
const changeUpdated = "updated"

// backedUpShape is the part of a container the change notice compares. A
// variable keeps its name and a digest of its value, so a changed password
// shows as changed without the password being kept.
func backedUpShape(in model.Inspect) model.Inspect {
	env := make([]string, 0, len(in.Config.Env))
	for _, e := range in.Config.Env {
		k, v, _ := strings.Cut(e, "=")
		sum := sha256.Sum256([]byte(v))
		env = append(env, k+"="+hex.EncodeToString(sum[:8]))
	}
	return model.Inspect{
		ID:    in.ID,
		Image: in.Image,
		Config: model.Config{
			Image: in.Config.Image,
			Env:   env,
		},
		HostConfig: model.HostConfig{
			Binds:        in.HostConfig.Binds,
			PortBindings: in.HostConfig.PortBindings,
		},
	}
}

func backedUpShapeJSON(in model.Inspect) string {
	b, err := json.Marshal(backedUpShape(in))
	if err != nil {
		return ""
	}
	return string(b)
}

// recordBackedUpShape keeps what a container looked like when its backup
// succeeded. A failed write costs the notice its baseline, not the backup.
func (s *Service) recordBackedUpShape(targetID, name string, in model.Inspect) {
	if err := s.store.SetBackedUpShape(targetID, backedUpShapeJSON(in)); err != nil {
		log.Printf("api: backup: the change notice of %q keeps its old baseline: %v", name, err) //nolint:gosec // G706: name is %q-quoted
	}
}

// backupBaseline is the shape of the container's last good backup. Before
// the first backup that records one, the stored definition stands in.
func backupBaseline(t store.Target, shape string) (model.Inspect, bool) {
	if shape != "" {
		var in model.Inspect
		if json.Unmarshal([]byte(shape), &in) == nil {
			return in, true
		}
	}
	if t.Definition == "" {
		return model.Inspect{}, false
	}
	var def containerDefinition
	if json.Unmarshal([]byte(t.Definition), &def) != nil {
		return model.Inspect{}, false
	}
	return backedUpShape(def.Inspect), true
}

// changesSinceBackup lists how a live container differs from its last good
// backup. A container that was not recreated since is skipped without an
// inspect: none of the compared settings can change without a new container,
// and the page lists every container at once.
func (s *Service) changesSinceBackup(ctx context.Context, live dockercli.ContainerInfo, t store.Target, shape string) []DefinitionChange {
	base, ok := backupBaseline(t, shape)
	if !ok || (base.ID != "" && base.ID == live.ID) {
		return nil
	}
	in, err := s.docker.Inspect(ctx, live.Name)
	if err != nil {
		log.Printf("api: list containers: inspecting %q for the change notice failed: %v", live.Name, err) //nolint:gosec // G706: name is %q-quoted
		return nil
	}
	return shapeChanges(base, backedUpShape(in))
}

func shapeChanges(backup, now model.Inspect) []DefinitionChange {
	out := containerChanges(backup, now)
	if containerImage(backup) == containerImage(now) && backup.Image != "" && now.Image != "" && backup.Image != now.Image {
		out = append([]DefinitionChange{{Field: "image", Change: changeUpdated, Now: containerImage(now)}}, out...)
	}
	return out
}
