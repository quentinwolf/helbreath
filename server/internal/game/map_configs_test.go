package game

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

func TestBuiltinMapConfigNamesAreNPCTypes(t *testing.T) {
	for _, name := range mapdata.BuiltinConfigs().Names() {
		for _, err := range checkConfigNames(mapdata.BuiltinConfigs().Get(name)) {
			t.Error(err)
		}
	}
}

func TestMapSpawnsFromConfig(t *testing.T) {
	c, errs := mapdata.ParseMapConfig("test", []byte(`
maximum-object = 15
waypoint = 1 66 38
npc = Howard 1 1 1 1 1 1 1 1 1 1 1 1
spot-mob-generator = 1 1 10 10 20 20 Slime 20
spot-mob-generator = 2 1 30 30 40 40 Orc 10
spot-mob-generator = 3 2 30 30 40 40 Orc 10
spot-mob-generator = 4 1 30 30 40 40 Orc 0
`))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	got := mapSpawns(c)
	if len(got) != 3 {
		t.Fatalf("want 2 spots (type 2 and max 0 skipped) and Howard, got %+v", got)
	}
	if got[0].NpcTypeID != 10 || got[0].Count != 10 || got[0].AreaX1 != 10 || got[0].AreaY2 != 20 {
		t.Errorf("slimes: 20 of the map's 30 against a cap of 15 = 10, in their rectangle: %+v", got[0])
	}
	if got[1].NpcTypeID != 14 || got[1].Count != 5 {
		t.Errorf("orcs: %+v", got[1])
	}
	if got[2].NpcTypeID != 20 || got[2].SpawnX != 66 || got[2].SpawnY != 38 {
		t.Errorf("Howard at his waypoint: %+v", got[2])
	}
}

func TestMapConfigEditsRespawnLive(t *testing.T) {
	if _, ok := npc.NpcTypes[15]; !ok {
		t.Skip("no ShopKeeper type")
	}
	defer mapdata.SetConfigs(mapdata.BuiltinConfigs())
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, mapdata.MapConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, mapdata.MapConfigDir, "test.txt"), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("waypoint = 1 10 10\nnpc = ShopKeeper-W 1 1 1 1 1 1 1 1 1 1 1 1\n")

	e := makeTestEngine()
	gm := makeTestGameMap(60, 60)
	e.maps[gm.Name] = gm
	e.SetMapEditsDir(dir)
	e.loadMapEdits()
	e.mapEdits.baseFromConfigs = true
	e.SpawnNPCsWithConfig(e.trackSpawns(DefaultSpawnConfig()))

	shopkeeperAt := func() (x, y, n int) {
		e.npcs.Range(func(_, v any) bool {
			np := v.(*npc.NPC)
			x, y = np.X, np.Y
			n++
			return true
		})
		return
	}
	if x, y, n := shopkeeperAt(); n != 1 || x != 10 || y != 10 {
		t.Fatalf("start: want the shopkeeper at 10,10, got %d NPCs at %d,%d", n, x, y)
	}

	time.Sleep(10 * time.Millisecond)
	write("waypoint = 1 40 42\nnpc = ShopKeeper-W 1 1 1 1 1 1 1 1 1 1 1 1\n")
	e.checkMapEdits()
	if x, y, n := shopkeeperAt(); n != 1 || x != 40 || y != 42 {
		t.Errorf("after the edit: want the shopkeeper moved to 40,42, got %d NPCs at %d,%d", n, x, y)
	}

	// A broken edit keeps the map as it was
	time.Sleep(10 * time.Millisecond)
	write("waypoint = 1 40 42\nnpc = Nobody 1 1 1 1 1 1 1 1 1 1 1 1\n")
	e.checkMapEdits()
	if x, y, n := shopkeeperAt(); n != 1 || x != 40 || y != 42 {
		t.Errorf("a broken file should change nothing, got %d NPCs at %d,%d", n, x, y)
	}
}
