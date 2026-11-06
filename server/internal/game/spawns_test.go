package game

import (
	"strings"
	"testing"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

func TestOriginalSpawnsReplaceHandWrittenMonsters(t *testing.T) {
	cfg := DefaultSpawnConfig()
	slimePits := 0
	for _, s := range cfg.Spawns {
		if !strings.EqualFold(s.MapName, "aresden") {
			continue
		}
		if s.NpcTypeID == 10 {
			if !s.HasArea() {
				t.Errorf("Aresden slimes should come from the original rectangles, got a hand-written spawn at %d,%d", s.SpawnX, s.SpawnY)
			}
			slimePits++
		}
	}
	if slimePits != 5 {
		t.Errorf("Aresden has 5 original slime pits, got %d", slimePits)
	}
	// Town NPCs stay
	found := false
	for _, s := range cfg.Spawns {
		if s.MapName == "cityhall_1" && s.NpcTypeID == npcCityHallOfficer {
			found = true
		}
	}
	if !found {
		t.Error("William should still spawn in Aresden's City Hall")
	}
}

func TestMapCapSharesOutTheSpawns(t *testing.T) {
	c := mapdata.Configs().Get("default")
	if c == nil || c.MaxObjects == 0 {
		t.Fatal("default.txt should have a maximum-object")
	}
	total := 0
	for _, s := range mapSpawns(c) {
		if !npc.IsShopNPC(s.NpcTypeID) {
			total += s.Count
		}
	}
	if total > c.MaxObjects || total < c.MaxObjects/2 {
		t.Errorf("the starter map should hold close to its cap of %d, got %d", c.MaxObjects, total)
	}
}

func TestMapNamedIgnoresCase(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(10, 10)
	gm.Name = "Cmdhall_1"
	e.maps[gm.Name] = gm
	if _, ok := e.mapNamed("cmdhall_1"); !ok {
		t.Error("map lookups should ignore case")
	}
}

func TestTownBuildingDoors(t *testing.T) {
	doors := mapdata.BuildTeleportConfig() // the map configs' doors
	// Both towns' shop, blacksmith and both warehouses, as the original map files
	for _, c := range []struct {
		town string
		x, y int
		dest string
	}{
		{"aresden", 101, 183, "wrhus_1"}, {"aresden", 217, 132, "arewrhus"},
		{"aresden", 167, 195, "bsmith_1"}, {"aresden", 126, 166, "gshop_1"},
		{"elvine", 197, 127, "wrhus_2"}, {"elvine", 87, 174, "elvwrhus"},
		{"elvine", 239, 107, "bsmith_2"}, {"elvine", 225, 151, "gshop_2"},
	} {
		dest, ok := doors.Nearest(c.town, c.x, c.y)
		if !ok || !strings.EqualFold(dest.DestMap, c.dest) {
			t.Errorf("%s %d,%d should lead into %s, got %+v", c.town, c.x, c.y, c.dest, dest)
		}
	}
	// A doorway tile next to a listed one still works
	if dest, ok := doors.Nearest("cityhall_1", 58, 41); !ok || dest.DestMap != "aresden" {
		t.Errorf("cityhall_1 58,41 should use the door next to it, got %+v", dest)
	}
}

func TestNPCsSpawnOnTheirOwnTiles(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	gm.Name = "test"
	e.maps["test"] = gm
	e.SpawnNPCsWithConfig(&SpawnConfig{Spawns: []SpawnEntry{
		{MapName: "test", NpcTypeID: 20, SpawnX: 66, SpawnY: 38},
		{MapName: "test", NpcTypeID: 25, SpawnX: 66, SpawnY: 38},
	}})
	tiles := map[[2]int]bool{}
	count := 0
	e.npcs.Range(func(_, v any) bool {
		n := v.(*npc.NPC)
		tiles[[2]int{n.X, n.Y}] = true
		count++
		return true
	})
	if count != 2 || len(tiles) != 2 {
		t.Errorf("two NPCs given the same spot should end up on different tiles, got %d NPCs on %d tiles", count, len(tiles))
	}
}

func TestBothWizardTowersHaveGandalf(t *testing.T) {
	cfg := DefaultSpawnConfig()
	for _, m := range []string{"wzdtwr_1", "wzdtwr_2"} {
		found := false
		for _, s := range cfg.Spawns {
			if s.MapName == m && s.NpcTypeID == npcWizard {
				found = true
			}
		}
		if !found {
			t.Errorf("Gandalf should spawn in %s (the original has him in both towers)", m)
		}
	}
}
