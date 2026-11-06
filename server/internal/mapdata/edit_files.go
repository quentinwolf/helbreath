package mapdata

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// walkable.json (the map editor's walkability edits) is read from a folder on
// disk when the server has one (normally this package's folder, so the editor's
// saves load while the server runs), otherwise from the copy built into the
// binary. Doors and spawns live in the map configs (maps/*.txt).
const WalkableFile = "walkable.json"

func readEditFile(dir, name string, builtin []byte) ([]byte, error) {
	if dir == "" {
		return builtin, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, name))
	if errors.Is(err, fs.ErrNotExist) {
		return builtin, nil
	}
	return data, err
}

// LoadWalkOverridesFrom parses dir/walkable.json, or the built-in copy when
// dir is "" or has no such file.
func LoadWalkOverridesFrom(dir string) (WalkOverrides, error) {
	data, err := readEditFile(dir, WalkableFile, walkOverridesJSON)
	if err != nil {
		return WalkOverrides{}, err
	}
	return ParseWalkOverrides(data)
}

// EditFileStamp identifies a version of dir/name (modified time and size), so
// a change can be noticed without reading the file. "" when there is no file.
func EditFileStamp(dir, name string) string {
	if dir == "" {
		return ""
	}
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d/%d", info.ModTime().UnixNano(), info.Size())
}
