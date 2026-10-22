package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/config"
	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/restic"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func restoreDefinitionService(t *testing.T, appdata []string) (*Service, repoRef) {
	t.Helper()
	st := newAliasLinkStore(t)
	def, err := json.Marshal(containerDefinition{Inspect: model.Inspect{Name: "sonarr"}, AppdataPaths: appdata})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpsertTarget(store.Target{ContainerName: "sonarr", Definition: string(def), AppdataPaths: appdata}); err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st, cfg: config.Config{HostMountRoot: "/host/user"}, engine: aliasRestoreEngine{}}
	return s, repoRef{repo: "s3:host/bucket/containers", mode: restic.Mode{}}
}

// A place that never got a container's data, such as an off-site the
// container is left out of, must not turn "latest" into a recreate that
// brings the container back empty and calls it restored.
func TestRestoreLatestRefusesAContainerWhoseDataThisPlaceLacks(t *testing.T) {
	s, ref := restoreDefinitionService(t, []string{"/host/user/appdata/sonarr"})
	plan, err := s.prepareRestoreIn(context.Background(), ref, "sonarr", "latest", true)
	if err == nil {
		t.Fatalf("latest = %+v; want a refusal", plan)
	}
}

func TestRestoreLatestRecreatesAStatelessContainer(t *testing.T) {
	s, ref := restoreDefinitionService(t, nil)
	plan, err := s.prepareRestoreIn(context.Background(), ref, "sonarr", "latest", true)
	if err != nil || !plan.recreateOnly {
		t.Fatalf("latest = %+v, %v; want a recreate from the definition", plan, err)
	}
}

func TestRestoreOfTheDefinitionRecreatesAContainerWithData(t *testing.T) {
	s, ref := restoreDefinitionService(t, []string{"/host/user/appdata/sonarr"})
	plan, err := s.prepareRestoreIn(context.Background(), ref, "sonarr", definitionOnly, true)
	if err != nil || !plan.recreateOnly || plan.snapshotID != "" || len(plan.appdataPaths) != 0 {
		t.Fatalf("definition = %+v, %v; want a recreate that restores no data", plan, err)
	}
}

func TestRestoreOfTheDefinitionNeedsOne(t *testing.T) {
	st := newAliasLinkStore(t)
	if _, err := st.UpsertTarget(store.Target{ContainerName: "sonarr", AppdataPaths: []string{"/host/user/appdata/sonarr"}}); err != nil {
		t.Fatal(err)
	}
	s := &Service{store: st, cfg: config.Config{HostMountRoot: "/host/user"}, engine: aliasRestoreEngine{}}
	if _, err := s.prepareRestoreIn(context.Background(), repoRef{repo: "s3:host/bucket/containers"}, "sonarr", definitionOnly, true); err == nil {
		t.Fatal("a recreate without a stored definition went through")
	}
}
