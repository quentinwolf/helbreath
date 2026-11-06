package mapdata

import (
	"strings"
	"testing"
)

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
	table := BuildTeleportTable()
	if len(table.Dests) == 0 {
		t.Fatal("no doors")
	}
	for m := range table.Exact {
		if m != strings.ToLower(m) {
			t.Errorf("exact map names should be lower case, got %q", m)
		}
	}
}

func TestExactDoorsFromConfig(t *testing.T) {
	a, _ := ParseMapConfig("A", []byte(`exact-doors = 1
teleport-loc = 5 5 B 1 1 0
teleport-loc = 6 6 nowhere 1 1 0`))
	b, _ := ParseMapConfig("B", []byte(`teleport-loc = 2 2 a 9 9 3`))
	table := TeleportTableFrom(MapConfigs{"a": a, "b": b})
	if !table.Exact["a"] || table.Exact["b"] {
		t.Errorf("exact maps: %v", table.Exact)
	}
	if d, ok := table.DoorAt(nil, "a", 5, 5); !ok || d.DestMap != "B" {
		t.Errorf("exact map A 5,5 -> B, got %+v %v", d, ok)
	}
	if _, ok := table.DoorAt(nil, "A", 6, 6); ok {
		t.Error("a door to a map without a config is left out")
	}
	if d := table.Dests[TeleportKey("B", 2, 2)]; d.DestMap != "A" || d.Dir != 3 {
		t.Errorf("destination names are spelled as the map's file: %+v", d)
	}
}
