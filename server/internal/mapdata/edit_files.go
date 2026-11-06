package mapdata

import (
	_ "embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// The map editor's files. The server reads them from a folder on disk when it
// has one (normally this package's folder, so the editor's saves load while the
// server runs) and otherwise uses the copies built into the binary.
const (
	TeleportsFile = "teleports.json"
	WalkableFile  = "walkable.json"
	SpawnsFile    = "spawns.json"
)

// spawns.json is parsed by the game package (spawn_overrides.go); it lives here
// with the other editor files.
//
//go:embed spawns.json
var spawnsJSON []byte

// ReadSpawnsFile is dir/spawns.json, or the built-in copy when dir is "" or has
// no such file.
func ReadSpawnsFile(dir string) ([]byte, error) {
	return readEditFile(dir, SpawnsFile, spawnsJSON)
}

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

// LoadTeleportOverridesFrom parses dir/teleports.json, or the built-in copy
// when dir is "" or has no such file.
func LoadTeleportOverridesFrom(dir string) (TeleportOverrides, error) {
	data, err := readEditFile(dir, TeleportsFile, teleportOverridesJSON)
	if err != nil {
		return TeleportOverrides{}, err
	}
	return ParseTeleportOverrides(data)
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
