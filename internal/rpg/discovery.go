package rpg

import (
	"os"
	"path/filepath"
)

type DirectoryDomainDiscoverer struct {
	BaseDirs []string
}

func (d *DirectoryDomainDiscoverer) DiscoverDomains(rootPath string) (map[string]string, error) {
	domains := make(map[string]string)

	// Always include the root as a fallback if no subdomains are found
	// domains["root"] = ""

	for _, base := range d.BaseDirs {
		basePath := filepath.Join(rootPath, base)
		
		entries, err := os.ReadDir(basePath)
		if err != nil {
			// Skip if base dir doesn't exist
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				name := entry.Name()
				// Store path relative to rootPath
				domains[name] = filepath.Join(base, name)
			}
		}
	}

	// If no domains found, fall back to root
	if len(domains) == 0 {
		domains["root"] = ""
	}

	return domains, nil
}
