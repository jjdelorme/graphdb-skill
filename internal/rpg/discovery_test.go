package rpg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirectoryDomainDiscoverer_DiscoverDomains(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup fake structure
	os.MkdirAll(filepath.Join(tmpDir, "internal", "auth"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "internal", "billing"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "cmd", "app"), 0755)

	discoverer := &DirectoryDomainDiscoverer{
		BaseDirs: []string{"internal", "cmd"},
	}

	domains, err := discoverer.DiscoverDomains(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverDomains failed: %v", err)
	}

	expected := map[string]string{
		"auth":    filepath.Join("internal", "auth"),
		"billing": filepath.Join("internal", "billing"),
		"app":     filepath.Join("cmd", "app"),
	}

	if len(domains) != 3 {
		t.Errorf("Expected 3 domains, got %d", len(domains))
	}

	for k, v := range expected {
		if domains[k] != v {
			t.Errorf("Domain %s: expected path %s, got %s", k, v, domains[k])
		}
	}
}

func TestDirectoryDomainDiscoverer_Fallback(t *testing.T) {
	tmpDir := t.TempDir()
	discoverer := &DirectoryDomainDiscoverer{
		BaseDirs: []string{"nonexistent"},
	}

	domains, err := discoverer.DiscoverDomains(tmpDir)
	if err != nil {
		t.Fatalf("DiscoverDomains failed: %v", err)
	}

	if len(domains) != 1 || domains["root"] != "" {
		t.Errorf("Expected fallback to root, got %v", domains)
	}
}
