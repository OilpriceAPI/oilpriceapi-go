package oilpriceapi

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestReleaseMetadataMatchesSDKVersion(t *testing.T) {
	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}

	firstRelease := regexp.MustCompile(`(?m)^## \[([^]]+)\]`).FindStringSubmatch(string(changelog))
	if len(firstRelease) != 2 {
		t.Fatal("CHANGELOG.md has no release heading")
	}
	if firstRelease[1] != Version {
		t.Fatalf("latest changelog version %q does not match SDK Version %q", firstRelease[1], Version)
	}
}

func TestWorkflowActionsAreCurrentAndHardened(t *testing.T) {
	entries, err := os.ReadDir(".github/workflows")
	if err != nil {
		t.Fatal(err)
	}

	refPattern := regexp.MustCompile(`(?m)^\s*(?:-\s*)?uses:\s*(actions/(?:checkout|setup-go))@([^\s#]+)\s*$`)
	found := map[string]int{"actions/checkout": 0, "actions/setup-go": 0}

	for _, entry := range entries {
		if entry.IsDir() || (filepath.Ext(entry.Name()) != ".yml" && filepath.Ext(entry.Name()) != ".yaml") {
			continue
		}
		path := filepath.Join(".github/workflows", entry.Name())
		contents, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		text := string(contents)
		refs := refPattern.FindAllStringSubmatch(text, -1)
		for _, ref := range refs {
			found[ref[1]]++
			if ref[2] != "v7" {
				t.Errorf("%s uses %s@%s; expected @v7", path, ref[1], ref[2])
			}
		}

		checkoutCount := strings.Count(text, "actions/checkout@")
		if hardenedCount := strings.Count(text, "persist-credentials: false"); hardenedCount != checkoutCount {
			t.Errorf("%s has %d checkout steps but %d hardened credential settings", path, checkoutCount, hardenedCount)
		}
		setupCount := strings.Count(text, "actions/setup-go@")
		if noCacheCount := strings.Count(text, "cache: false"); noCacheCount != setupCount {
			t.Errorf("%s has %d setup-go steps but %d explicit cache opt-outs", path, setupCount, noCacheCount)
		}
	}

	for action, count := range found {
		if count == 0 {
			t.Errorf("workflow set has no %s references", action)
		}
	}
}
