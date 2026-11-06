package game

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

func TestEmbeddedSpawnOverridesParse(t *testing.T) {
	if _, err := LoadSpawnOverridesFrom(""); err != nil {
		t.Fatalf("spawns.json (written by the map editor) is broken: %v", err)
	}
}

func TestParseSpawnOverridesRejectsBadFiles(t *testing.T) {
	for name, data := range map[string]string{
		"not json":   `{`,
		"no version": `{"maps": {}}`,
		"bad npc":    `{"version": 1, "maps": {"elvine": [{"npc": 999999, "x": 1, "y": 1}]}}`,
		"bad dir":    `{"version": 1, "maps": {"elvine": [{"npc": 15, "x": 1, "y": 1, "dir": 9}]}}`,
		"bad area":   `{"version": 1, "maps": {"elvine": [{"npc": 10, "area": [10, 10, 5, 20]}]}}`,
		"map twice":  `{"version": 1, "maps": {"elvine": [], "Elvine": []}}`,
	} {
		if _, err := ParseSpawnOverrides([]byte(data)); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestSpawnOverridesReplaceAMapsSpawns(t *testing.T) {
	base := &SpawnConfig{Spawns: []SpawnEntry{
		{MapName: "elvine", NpcTypeID: 10, Count: 5, SpawnX: 1, SpawnY: 1},
		{MapName: "Elvine", NpcTypeID: 22, Count: 5, SpawnX: 2, SpawnY: 2},
		{MapName: "aresden", NpcTypeID: 10, Count: 5, SpawnX: 3, SpawnY: 3},
	}}
	o, err := ParseSpawnOverrides([]byte(`{"version": 1, "maps": {"elvine": [
		{"npc": 15, "x": 40, "y": 41, "dir": 5, "note": "Shop"},
		{"npc": 10, "count": 3, "area": [10, 11, 20, 21], "respawn": 30}]}}`))
	if err != nil {
		t.Fatal(err)
	}
	cfg := o.Apply(base)
	elvine := spawnsOn(cfg, "ELVINE")
	if len(elvine) != 2 || len(spawnsOn(cfg, "aresden")) != 1 {
		t.Fatalf("elvine should have exactly the 2 listed spawns, aresden its own: %+v", cfg.Spawns)
	}
	if s := elvine[0]; s.NpcTypeID != 15 || s.SpawnX != 40 || s.Dir != 5 {
		t.Errorf("shopkeeper: %+v", s)
	}
	if s := elvine[1]; !s.HasArea() || s.AreaX2 != 20 || s.RespawnDelay != 30 || s.Count != 3 {
		t.Errorf("slimes: %+v", s)
	}
}

func TestSpawnEditsRespawnWhileRunning(t *testing.T) {
	if _, ok := npc.NpcTypes[15]; !ok {
		t.Skip("no ShopKeeper type")
	}
	dir := t.TempDir()
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, mapdata.SpawnsFile), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(`{"version": 1, "maps": {}}`)

	e := makeTestEngine()
	gm := makeTestGameMap(60, 60)
	e.maps[gm.Name] = gm
	e.SetMapEditsDir(dir)
	e.loadMapEdits()
	base := &SpawnConfig{Spawns: []SpawnEntry{{MapName: "test", NpcTypeID: 15, SpawnX: 10, SpawnY: 10}}}
	e.mapEdits.baseSpawns = base
	e.SpawnNPCsWithConfig(e.loadSpawnEdits(base))

	onMap := func() []*npc.NPC {
		var out []*npc.NPC
		e.npcs.Range(func(_, v any) bool {
			out = append(out, v.(*npc.NPC))
			return true
		})
		return out
	}
	if n := onMap(); len(n) != 1 || n[0].X != 10 {
		t.Fatalf("start: one shopkeeper at 10,10, got %d NPCs, first at %d,%d", len(n), n[0].X, n[0].Y)
	}
	summon := npc.NewNPC(9999, npc.NpcTypes[15], "test", 30, 30)
	summon.Summoned = true
	e.npcs.Store(int32(9999), summon)

	time.Sleep(10 * time.Millisecond)
	write(`{"version": 1, "maps": {"test": [{"npc": 15, "x": 40, "y": 42, "dir": 3}]}}`)
	e.checkMapEdits()
	var moved, summons int
	for _, n := range onMap() {
		if n.Summoned {
			summons++
		} else if n.X == 40 && n.Y == 42 && n.Direction == 3 {
			moved++
		} else {
			t.Errorf("unexpected NPC left behind: %+v at %d,%d", n.Type.Name, n.X, n.Y)
		}
	}
	if moved != 1 || summons != 1 {
		t.Errorf("want the shopkeeper moved to 40,42 facing 3 and the summon kept, got moved=%d summons=%d", moved, summons)
	}
	if gm.Tiles[10][10].Owner != 0 {
		t.Error("the old spot should be free")
	}

	// Taking the map out of spawns.json brings back the built-in spawn.
	time.Sleep(10 * time.Millisecond)
	write(`{"version": 1, "maps": {}}`)
	e.checkMapEdits()
	back := 0
	for _, n := range onMap() {
		if !n.Summoned && n.X == 10 && n.Y == 10 {
			back++
		}
	}
	if back != 1 {
		t.Error("the built-in shopkeeper should be back at 10,10")
	}
}
