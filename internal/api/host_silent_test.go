package api_test

import (
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/junkerderprovinz/bombvault/internal/dockercli"
	"github.com/junkerderprovinz/bombvault/internal/store"
	"github.com/junkerderprovinz/bombvault/internal/virshcli"
)

func TestOneContainerIsTheRowOfTheList(t *testing.T) {
	docker := &fakeServiceDocker{listOut: []dockercli.ContainerInfo{{Name: "plex", State: "running", Image: "plexinc/pms"}}}
	h, st, _ := newItemsRouter(t, docker, fakeVirsh{})
	enableEveryKind(t, st, nil)
	seedTarget(t, st, "plex")
	seedTarget(t, st, "sonarr")

	_, list := doJSON(t, h, http.MethodGet, "/api/containers", "")
	rows, _ := list["containers"].([]any)
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		w, body := doJSON(t, h, http.MethodGet, "/api/containers/"+row["name"].(string), "")
		if w.Code != http.StatusOK || body["ok"] != true || body["hostAnswered"] != true {
			t.Fatalf("GET /api/containers/%v: %d %v", row["name"], w.Code, body)
		}
		if !reflect.DeepEqual(body["container"], row) {
			t.Fatalf("%v read on its own is\n%v\nand in the list\n%v", row["name"], body["container"], row)
		}
	}

	w, body := doJSON(t, h, http.MethodGet, "/api/containers/radarr", "")
	if w.Code != http.StatusNotFound || body["code"] != "not-found" {
		t.Fatalf("a container nobody knows: %d %v, want a 404", w.Code, body)
	}
}

func TestOneContainerIsServedFromItsEntryWhileDockerIsSilent(t *testing.T) {
	docker := &fakeServiceDocker{listErr: errors.New("docker: socket unreachable")}
	h, st, _ := newItemsRouter(t, docker, fakeVirsh{})
	enableEveryKind(t, st, nil)
	if _, err := st.UpsertTarget(store.Target{
		ContainerName: "plex", IncludeInSchedule: true,
		PreHook: "/boot/before.sh", Excludes: []string{"cache/**"}, StopContainers: []string{"sonarr"},
	}); err != nil {
		t.Fatal(err)
	}

	if _, body := doJSON(t, h, http.MethodGet, "/api/containers", ""); body["ok"] != false {
		t.Fatalf("the list answers %v without Docker, want it to fail as before", body)
	}

	w, body := doJSON(t, h, http.MethodGet, "/api/containers/plex", "")
	if w.Code != http.StatusOK || body["ok"] != true || body["hostAnswered"] != false {
		t.Fatalf("GET /api/containers/plex without Docker: %d %v", w.Code, body)
	}
	c, _ := body["container"].(map[string]any)
	if c["state"] != "" {
		t.Fatalf("state = %q though Docker said nothing", c["state"])
	}
	if c["preHook"] != "/boot/before.sh" || !reflect.DeepEqual(c["excludes"], []any{"cache/**"}) ||
		!reflect.DeepEqual(c["stopContainers"], []any{"sonarr"}) || c["includeInSchedule"] != true {
		t.Fatalf("the row leaves out what the entry keeps: %v", c)
	}
	if _, placed := c["placement"].(map[string]any); !placed {
		t.Fatalf("the row has no placement: %v", c)
	}

	w, body = doJSON(t, h, http.MethodGet, "/api/containers/radarr", "")
	if w.Code != http.StatusOK || body["ok"] != false {
		t.Fatalf("a container without an entry while Docker is silent: %d %v, want a failure and no 404", w.Code, body)
	}
}

func TestOneVMIsServedFromItsEntryWhileLibvirtIsSilent(t *testing.T) {
	virsh := &countingVirsh{listErr: errors.New("ssh: could not resolve hostname")}
	h, st, _ := newItemsRouter(t, &fakeServiceDocker{}, virsh)
	enableEveryKind(t, st, nil)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "Windows 11", Method: "graceful", IncludeInSchedule: true}); err != nil {
		t.Fatal(err)
	}

	if _, body := doJSON(t, h, http.MethodGet, "/api/vms", ""); body["ok"] != false {
		t.Fatalf("the list answers %v without libvirt, want it to fail as before", body)
	}

	w, body := doJSON(t, h, http.MethodGet, "/api/vms/Windows%2011", "")
	if w.Code != http.StatusOK || body["ok"] != true || body["hostAnswered"] != false {
		t.Fatalf("GET /api/vms/Windows 11 without libvirt: %d %v", w.Code, body)
	}
	vm, _ := body["vm"].(map[string]any)
	if vm["libvirtName"] != "Windows 11" || vm["state"] != "" || vm["includeInSchedule"] != true {
		t.Fatalf("the row is %v, want the stored entry with an empty state", vm)
	}

	w, body = doJSON(t, h, http.MethodGet, "/api/vms/debian", "")
	if w.Code != http.StatusOK || body["ok"] != false {
		t.Fatalf("a VM without an entry while libvirt is silent: %d %v, want a failure and no 404", w.Code, body)
	}
}

func TestOneVMIsTheRowOfTheList(t *testing.T) {
	virsh := listVMsVirsh{vms: []virshcli.VMInfo{{Name: "win11", State: "running"}}}
	h, st, _ := newItemsRouter(t, &fakeServiceDocker{}, virsh)
	enableEveryKind(t, st, nil)
	if _, err := st.UpsertVMTarget(store.VMTarget{Name: "debian", Method: "graceful"}); err != nil {
		t.Fatal(err)
	}

	_, list := doJSON(t, h, http.MethodGet, "/api/vms", "")
	rows, _ := list["vms"].([]any)
	if len(rows) != 2 {
		t.Fatalf("listed %v, want win11 and debian", rows)
	}
	for _, raw := range rows {
		row, _ := raw.(map[string]any)
		w, body := doJSON(t, h, http.MethodGet, "/api/vms/"+row["libvirtName"].(string), "")
		if w.Code != http.StatusOK || body["hostAnswered"] != true || !reflect.DeepEqual(body["vm"], row) {
			t.Fatalf("GET /api/vms/%v: %d %v, want the list's row %v", row["libvirtName"], w.Code, body, row)
		}
	}
	if w, _ := doJSON(t, h, http.MethodGet, "/api/vms/arch", ""); w.Code != http.StatusNotFound {
		t.Fatalf("a VM nobody knows answers %d, want a 404", w.Code)
	}
}
