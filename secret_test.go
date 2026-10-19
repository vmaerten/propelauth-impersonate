package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchAPIKeyNeedsTheProfileRegion(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("[profile noregion]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", filepath.Join(dir, "config"))
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", filepath.Join(dir, "credentials"))
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_PROFILE", "noregion")

	_, err := fetchAPIKey(context.Background(), "secret", "")
	if err == nil || !strings.Contains(err.Error(), "profile noregion has no region") {
		t.Errorf("got %v, want the active profile named as having no region", err)
	}
}
