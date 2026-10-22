package api

import (
	"slices"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/model"
	"github.com/junkerderprovinz/bombvault/internal/store"
)

func composeMember(project, service string, dependsOn string) model.Inspect {
	labels := map[string]string{"com.docker.compose.project": project, "com.docker.compose.service": service}
	if dependsOn != "" {
		labels["com.docker.compose.depends_on"] = dependsOn
	}
	return model.Inspect{Config: model.Config{Labels: labels}}
}

// A container the target's compose file depends on, or one a legacy link
// points at, is what the target needs before it starts. Anything else in the
// list may as well need the target.
func TestDependsOnContainerReadsTheTargetsOwnDependencies(t *testing.T) {
	nextcloud := composeMember("cloud", "app", "db:service_healthy:true,redis:service_started:false")
	if !dependsOnContainer(nextcloud, "cloud-db-1", composeMember("cloud", "db", "")) {
		t.Fatal("the database the app depends on must start before it")
	}
	if dependsOnContainer(nextcloud, "cloud-cron-1", composeMember("cloud", "cron", "app:service_started:false")) {
		t.Fatal("a container that depends on the app is not one the app needs")
	}
	if dependsOnContainer(nextcloud, "other-db-1", composeMember("other", "db", "")) {
		t.Fatal("a service of the same name in another project is not this one")
	}
	if dependsOnContainer(model.Inspect{}, "authelia", model.Inspect{}) {
		t.Fatal("without compose labels nothing says the target needs it")
	}
	linked := model.Inspect{HostConfig: model.HostConfig{Links: []string{"/mariadb:/nextcloud/db"}}}
	if !dependsOnContainer(linked, "mariadb", model.Inspect{}) {
		t.Fatal("a legacy link names a container the target needs")
	}
}

// Unraid templates on a custom network reach their database by name, which no
// label records, so a database listed under an app starts before it.
func TestADatabaseInTheListStartsBeforeTheApp(t *testing.T) {
	app := model.Inspect{Config: model.Config{Image: "authelia/authelia:latest"}}
	mariadb := model.Inspect{Config: model.Config{Image: "lscr.io/linuxserver/mariadb:latest"}}
	adminer := model.Inspect{Config: model.Config{Image: "adminer:latest"}}
	if !startsBefore(app, "mariadb", mariadb) {
		t.Fatal("the database of a template app starts after the app")
	}
	if startsBefore(app, "adminer", adminer) {
		t.Fatal("an app that is no database started before the target")
	}
	if startsBefore(mariadb, "phpmyadmin", adminer) || startsBefore(mariadb, "other-db", mariadb) {
		t.Fatal("what a database's own list holds started before the database")
	}
}

// The hint suggests listing the database under its app, so an import into the
// database stops that app as well as the database's own list.
func TestAnImportStopsTheAppsThatListTheDatabase(t *testing.T) {
	svc, st := newUpdateTestSvc(t)
	for _, tg := range []store.Target{
		{ContainerName: "nextcloud", StopContainers: []string{"mariadb", "redis"}},
		{ContainerName: "authelia", StopContainers: []string{"mariadb"}},
		{ContainerName: "mariadb", StopContainers: []string{"phpmyadmin"}},
		{ContainerName: "plex", StopContainers: []string{"tautulli"}},
	} {
		if _, err := st.UpsertTarget(tg); err != nil {
			t.Fatal(err)
		}
		if err := st.SetStopContainers(tg.ContainerName, tg.StopContainers); err != nil {
			t.Fatal(err)
		}
	}
	got := svc.importDependentNames("mariadb")
	if want := []string{"phpmyadmin", "authelia", "nextcloud"}; !slices.Equal(got, want) {
		t.Fatalf("importDependentNames = %v, want %v", got, want)
	}
}
