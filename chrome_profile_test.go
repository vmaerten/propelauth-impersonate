package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindChromeProfile(t *testing.T) {
	localState := filepath.Join(t.TempDir(), "Local State")
	state := `{"profile": {"info_cache": {"Default": {"name": "Personal"}, "Profile 2": {"name": "Work"}}}}`
	if err := os.WriteFile(localState, []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Work", "Profile 2"} {
		if got, err := findChromeProfile(localState, name); err != nil || got != (chromeProfile{name: "Work", dir: "Profile 2"}) {
			t.Errorf("%s: got %+v, %v; want the Work profile in Profile 2", name, got, err)
		}
	}
	if _, err := findChromeProfile(localState, "Test"); err == nil || !strings.Contains(err.Error(), `"Personal", "Work"`) {
		t.Errorf("got %v, want an error listing the profiles", err)
	}
}
