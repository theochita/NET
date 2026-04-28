package tftp

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

var errPathEscape = errors.New("tftp: path escapes root")

// resolvePath sanitises a client-supplied filename and returns its absolute
// location under root. It strips ".." and absolute prefixes, joins against
// root, and resolves symlinks — rejecting any final path that escapes root.
// Non-existent targets are allowed (WRQ creates new files).
func resolvePath(root, name string) (string, error) {
	name = filepath.FromSlash(name)
	clean := filepath.Clean(string(filepath.Separator) + name)
	clean = strings.TrimPrefix(clean, string(filepath.Separator))
	full := filepath.Join(root, clean)

	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		if os.IsNotExist(err) {
			return full, nil
		}
		return "", err
	}

	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	if real != rootReal && !strings.HasPrefix(real, rootReal+string(filepath.Separator)) {
		return "", errPathEscape
	}
	return full, nil
}
