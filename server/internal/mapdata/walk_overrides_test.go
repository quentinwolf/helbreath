package mapdata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedWalkOverridesParse(t *testing.T) {
	if _, err := LoadWalkOverrides(); err != nil {
		t.Fatalf("walkable.json (written by the map editor) is broken: %v", err)
	}
}

func TestParseWalkOverridesRejectsBadFiles(t *testing.T) {
	for name, data := range map[string]string{
		"not json":   `{`,
		"no version": `{"maps": {}}`,
		"tile twice": `{"version": 1, "maps": {"elvine": {"block": [[1,2]], "open": [[1,2]]}}}`,
		"map twice":  `{"version": 1, "maps": {"elvine": {}, "Elvine": {}}}`,
	} {
		if _, err := ParseWalkOverrides([]byte(data)); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestWalkOverridesApply(t *testing.T) {
	gm := &GameMap{Name: "Elvine", Width: 10, Height: 10, Tiles: make([][]Tile, 10)}
	for y := range gm.Tiles {
		gm.Tiles[y] = make([]Tile, 10)
		for x := range gm.Tiles[y] {
			gm.Tiles[y][x].Walkable = x != 5 // column 5 blocked
			gm.Tiles[y][x].BaseWalkable = x != 5
		}
	}
	o, err := ParseWalkOverrides([]byte(`{"version": 1, "maps": {"elvine": {"block": [[1,1],[2,2],[50,50]], "open": [[5,3]]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if n := o.Reapply(gm); n != 3 {
		t.Errorf("changed %d tiles, want 3 (the tile off the map is skipped)", n)
	}
	if gm.Tiles[1][1].Walkable || gm.Tiles[2][2].Walkable {
		t.Error("blocked tiles should not be walkable")
	}
	if !gm.Tiles[3][5].Walkable || gm.Tiles[4][5].Walkable {
		t.Error("only the opened tile should become walkable")
	}
	if n := o.Reapply(gm); n != 0 {
		t.Errorf("reapplying the same edits should change nothing, changed %d", n)
	}
	// Edits taken out of the file go back to the .amd walkability.
	if n := (WalkOverrides{Version: 1}).Reapply(gm); n != 3 {
		t.Errorf("removing the edits changed %d tiles, want 3", n)
	}
	if !gm.Tiles[1][1].Walkable || gm.Tiles[3][5].Walkable {
		t.Error("tiles should be back as the .amd has them")
	}
}

func TestEditFilesFromDisk(t *testing.T) {
	dir := t.TempDir()
	if EditFileStamp(dir, WalkableFile) != "" {
		t.Error("no file, no stamp")
	}
	// Missing file: the built-in copy.
	if _, err := LoadWalkOverridesFrom(dir); err != nil {
		t.Fatal(err)
	}
	data := `{"version": 1, "maps": {"elvine": {"open": [[1,1]]}}}`
	if err := os.WriteFile(filepath.Join(dir, WalkableFile), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := LoadWalkOverridesFrom(dir)
	if err != nil || len(o.Maps["elvine"].Open) != 1 {
		t.Fatalf("should read the file on disk: %+v %v", o, err)
	}
	if EditFileStamp(dir, WalkableFile) == "" {
		t.Error("file on disk should have a stamp")
	}
}
