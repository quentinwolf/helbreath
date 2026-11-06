package game

import (
	"testing"

	"github.com/juanrossi/hbonline/server/internal/npc"
)

func TestRandomMobLevelOne(t *testing.T) {
	seen := map[string]int{}
	for i := 0; i < 2000; i++ {
		code, _ := randomMobCode(1, iDice)
		seen[randomMobNames[code]]++
	}
	for name := range seen {
		switch name {
		case "Slime", "Giant-Ant", "Rabbit", "Cat", "Orc":
		default:
			t.Errorf("level 1 (towns) rolled %q", name)
		}
	}
	if seen["Rabbit"] < seen["Slime"] {
		t.Errorf("rabbits are the most common in town (45%%): %v", seen)
	}
}

func TestRandomMobNamesResolve(t *testing.T) {
	// Every monster the level tables can roll should exist here, except the ones
	// this port doesn't have (they're skipped).
	missing := map[string]bool{}
	for level := 1; level <= 21; level++ {
		for i := 0; i < 500; i++ {
			code, _ := randomMobCode(level, iDice)
			name, ok := randomMobNames[code]
			if !ok {
				name = "Orc"
			}
			if _, err := npc.TypeForConfigName(name); err != nil {
				missing[name] = true
			}
		}
	}
	for name := range missing {
		t.Logf("not spawned (no such monster here): %s", name)
	}
	if len(missing) > 3 {
		t.Errorf("too many monsters missing: %v", missing)
	}
}

func TestRandomMobGeneratorFillsAndRefills(t *testing.T) {
	useTestConfig(t, "maximum-object = 40\nrandom-mob-generator = 1 1\nnpc-avoidrect = 1 15 15 40 40\n")
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	for i := 0; i < 200; i++ {
		e.runRandomMobs()
	}
	count := 0
	var one *npc.NPC
	e.npcs.Range(func(_, v any) bool {
		n := v.(*npc.NPC)
		count++
		one = n
		if !n.RandomMob {
			t.Error("generator NPCs should be marked RandomMob")
		}
		return true
	})
	// Up to maximum-object - 30 = 10, plus the last group's followers
	if count < 10 || count > 14 {
		t.Errorf("want about 10 random monsters, got %d", count)
	}
	// A dead one is removed (the generator makes a new one instead of respawning it)
	one.State = npc.StateDead
	e.processNPCTick(one, one.DeathTime)
	if _, ok := e.npcs.Load(one.ObjectID); ok {
		t.Error("a dead random monster should be removed")
	}
}

func TestRandomMobSpotAvoidsRect(t *testing.T) {
	useTestConfig(t, "npc-avoidrect = 1 0 0 99 60\n")
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	c := testConfig(t)
	for i := 0; i < 200; i++ {
		if x, y, ok := randomMobSpot(gm, c); ok && c.InNpcAvoid(x, y) {
			t.Fatalf("spawned inside the avoid rectangle at %d,%d", x, y)
		}
	}
}
