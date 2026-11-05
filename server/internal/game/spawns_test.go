package game

import (
	"strings"
	"testing"
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
	total := 0
	for _, s := range cappedOriginalSpawns() {
		if strings.EqualFold(s.MapName, "default") {
			total += s.Count
		}
	}
	if total > originalMaxObjects["default"] || total < originalMaxObjects["default"]/2 {
		t.Errorf("the starter map should hold close to its cap of %d, got %d", originalMaxObjects["default"], total)
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
