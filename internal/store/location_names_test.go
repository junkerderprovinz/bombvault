package store_test

import (
	"reflect"
	"testing"
)

func TestALocationNameIsKeptReplacedAndGivenBack(t *testing.T) {
	r := newRepo(t)
	if err := r.SetLocationName("abc", "  Office NAS "); err != nil {
		t.Fatal(err)
	}
	if err := r.SetLocationName("def", "Cellar"); err != nil {
		t.Fatal(err)
	}
	if err := r.SetLocationName("abc", "DXP4800"); err != nil {
		t.Fatal(err)
	}
	names, err := r.LocationNames()
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"abc": "DXP4800", "def": "Cellar"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}

	if err := r.SetLocationName("abc", " "); err != nil {
		t.Fatal(err)
	}
	if names, _ = r.LocationNames(); !reflect.DeepEqual(names, map[string]string{"def": "Cellar"}) {
		t.Fatalf("an empty name must give the default back, got %v", names)
	}
}
