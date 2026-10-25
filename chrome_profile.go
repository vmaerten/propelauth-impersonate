package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// chromeProfile is one of the user's own Chrome profiles; its zero value stands for the
// dedicated profile.
type chromeProfile struct {
	name string
	// dir is the profile's directory under Chrome's user data, which --profile-directory wants.
	dir string
}

// lookupChromeProfile finds the Chrome profile called name, the dedicated one when name is empty.
func lookupChromeProfile(name string) (chromeProfile, error) {
	if name == "" {
		return chromeProfile{}, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return chromeProfile{}, err
	}
	return findChromeProfile(filepath.Join(configDir, "Google", "Chrome", "Local State"), name)
}

// findChromeProfile looks name up, a display name or a directory, in Chrome's Local State file,
// which maps each profile directory to its display name.
func findChromeProfile(localState, name string) (chromeProfile, error) {
	data, err := os.ReadFile(localState)
	if err != nil {
		return chromeProfile{}, fmt.Errorf("reading the Chrome profiles: %w", err)
	}
	var state struct {
		Profile struct {
			InfoCache map[string]struct {
				Name string `json:"name"`
			} `json:"info_cache"`
		} `json:"profile"`
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return chromeProfile{}, fmt.Errorf("decoding %s: %w", localState, err)
	}
	var names []string
	for dir, info := range state.Profile.InfoCache {
		if info.Name == name || dir == name {
			return chromeProfile{name: info.Name, dir: dir}, nil
		}
		names = append(names, strconv.Quote(info.Name))
	}
	slices.Sort(names)
	return chromeProfile{}, fmt.Errorf("no Chrome profile called %q, among %s", name, strings.Join(names, ", "))
}

// openInChromeProfile opens url in one of the user's own Chrome profiles. Chrome allows no
// debugging there, so the Impersonate click is left to the user.
func openInChromeProfile(ctx context.Context, profile chromeProfile, url string) error {
	// -n hands the arguments to Chrome even when it is already running.
	cmd := exec.CommandContext(ctx, "open", "-na", "Google Chrome", "--args", "--profile-directory="+profile.dir, url)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("opening Chrome: %w: %s", err, out)
	}
	return nil
}
