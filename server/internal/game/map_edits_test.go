package game

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
)

func TestMapEditsReloadWhileRunning(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(mapdata.TeleportsFile, `{"version": 1, "maps": {}}`)
	write(mapdata.WalkableFile, `{"version": 1, "maps": {}}`)

	e := makeTestEngine()
	gm := makeTestGameMap(20, 20)
	for y := range gm.Tiles {
		for x := range gm.Tiles[y] {
			gm.Tiles[y][x].BaseWalkable = true
		}
	}
	e.maps[gm.Name] = gm
	e.SetMapEditsDir(dir)
	e.loadMapEdits()
	if _, ok := e.doorAt(gm, "test", 5, 5); ok {
		t.Fatal("no doors yet")
	}

	// The editor saves both files; the next check picks them up.
	time.Sleep(10 * time.Millisecond) // a new modified time on coarse clocks
	write(mapdata.TeleportsFile, `{"version": 1, "maps": {"test": [{"tiles": [[5,5]], "to": {"map": "test", "x": 1, "y": 1}}]}}`)
	write(mapdata.WalkableFile, `{"version": 1, "maps": {"test": {"block": [[3,3]]}}}`)
	e.checkMapEdits()
	if _, ok := e.doorAt(gm, "test", 5, 5); !ok {
		t.Error("the new door should work without a restart")
	}
	if gm.Tiles[3][3].Walkable {
		t.Error("the blocked tile should be blocked without a restart")
	}

	// A broken save keeps what was loaded.
	time.Sleep(10 * time.Millisecond)
	write(mapdata.WalkableFile, `{"version": 1, "maps": {"test": {"block": [[3,3]], "open": [[3,3]]}}}`)
	e.checkMapEdits()
	if gm.Tiles[3][3].Walkable {
		t.Error("a broken walkable.json should not undo the loaded edits")
	}

	// Fixed again, with the edit removed: the tile goes back to the .amd.
	time.Sleep(10 * time.Millisecond)
	write(mapdata.WalkableFile, `{"version": 1, "maps": {}}`)
	e.checkMapEdits()
	if !gm.Tiles[3][3].Walkable {
		t.Error("removing the edit should make the tile walkable again")
	}
}
