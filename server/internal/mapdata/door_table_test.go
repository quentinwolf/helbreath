package mapdata

import (
	"strings"
	"testing"
)

func TestEmbeddedTeleportOverridesParse(t *testing.T) {
	if _, err := LoadTeleportOverrides(); err != nil {
		t.Fatalf("teleports.json (written by the map editor) is broken: %v", err)
	}
}

func TestParseTeleportOverridesRejectsBadFiles(t *testing.T) {
	for name, data := range map[string]string{
		"not json":    `{`,
		"no version":  `{"maps": {}}`,
		"no dest map": `{"version": 1, "maps": {"elvine": [{"tiles": [[1,2]], "to": {"x": 1, "y": 1}}]}}`,
		"no tiles":    `{"version": 1, "maps": {"elvine": [{"tiles": [], "to": {"map": "gshop_2", "x": 1, "y": 1}}]}}`,
		"bad dir":     `{"version": 1, "maps": {"elvine": [{"tiles": [[1,2]], "to": {"map": "gshop_2", "x": 1, "y": 1, "dir": 9}}]}}`,
		"shared tile": `{"version": 1, "maps": {"elvine": [{"tiles": [[1,2]], "to": {"map": "a", "x": 1, "y": 1}}, {"tiles": [[1,2]], "to": {"map": "b", "x": 1, "y": 1}}]}}`,
		"map twice":   `{"version": 1, "maps": {"elvine": [], "Elvine": []}}`,
	} {
		if _, err := ParseTeleportOverrides([]byte(data)); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestTeleportOverridesReplaceAMapsDoors(t *testing.T) {
	tc := TeleportConfig{
		TeleportKey("elvine", 10, 10):  {DestMap: "gshop_2", DestX: 1, DestY: 1},
		TeleportKey("elvine", 20, 20):  {DestMap: "wrhus_2", DestX: 2, DestY: 2},
		TeleportKey("aresden", 10, 10): {DestMap: "gshop_1", DestX: 3, DestY: 3},
	}
	o, err := ParseTeleportOverrides([]byte(`{"version": 1, "maps": {"Elvine": [
		{"name": "Shop", "tiles": [[11,10],[12,10]], "to": {"map": "gshop_2", "x": 40, "y": 30, "dir": 1}}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	exact := o.Apply(tc)
	if !exact["elvine"] || exact["aresden"] {
		t.Errorf("only elvine should be exact, got %v", exact)
	}
	for _, key := range []string{TeleportKey("elvine", 10, 10), TeleportKey("elvine", 20, 20)} {
		if _, ok := tc[key]; ok {
			t.Errorf("built-in door %s should be gone", key)
		}
	}
	if d := tc[TeleportKey("elvine", 12, 10)]; d.DestMap != "gshop_2" || d.DestX != 40 || d.Dir != 1 {
		t.Errorf("new door tile: got %+v", d)
	}
	if _, ok := tc[TeleportKey("aresden", 10, 10)]; !ok {
		t.Error("other maps keep their built-in doors")
	}
}

func TestDoorAt(t *testing.T) {
	gm := &GameMap{Name: "elvine", Width: 30, Height: 30, Tiles: make([][]Tile, 30)}
	for y := range gm.Tiles {
		gm.Tiles[y] = make([]Tile, 30)
	}
	gm.Tiles[5][5].Teleport = true // flagged, door entry one tile away
	gm.Tiles[8][8].Teleport = true // flagged, no door anywhere near
	dests := TeleportConfig{TeleportKey("elvine", 6, 5): {DestMap: "gshop_2"}}

	built := TeleportTable{Dests: dests, Exact: map[string]bool{}}
	if _, ok := built.DoorAt(gm, "elvine", 5, 5); !ok {
		t.Error("built-in map: flagged tile should borrow the door next to it")
	}
	if _, ok := built.DoorAt(gm, "elvine", 6, 5); ok {
		t.Error("built-in map: an entry on an unflagged tile should not fire")
	}
	if _, ok := built.DoorAt(gm, "elvine", 8, 8); ok {
		t.Error("built-in map: flagged tile without a door nearby should not fire")
	}

	edited := TeleportTable{Dests: dests, Exact: map[string]bool{"elvine": true}}
	if _, ok := edited.DoorAt(gm, "Elvine", 6, 5); !ok {
		t.Error("edited map: listed tile should fire without the flag")
	}
	if _, ok := edited.DoorAt(gm, "elvine", 5, 5); ok {
		t.Error("edited map: unlisted tiles should not borrow a door")
	}
}

func TestBuildTeleportTable(t *testing.T) {
	table, err := BuildTeleportTable()
	if err != nil {
		t.Fatal(err)
	}
	if len(table.Dests) == 0 {
		t.Fatal("no doors")
	}
	for m := range table.Exact {
		if m != strings.ToLower(m) {
			t.Errorf("exact map names should be lower case, got %q", m)
		}
	}
}
