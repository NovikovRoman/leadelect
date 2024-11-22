package node

import "path/filepath"

// absPath converts a relative path to an absolute path.
func absPath(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}

	rel, _ = filepath.Abs(rel)
	return rel
}
