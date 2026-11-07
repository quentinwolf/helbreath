package game

import (
	"testing"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
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
	// 20% of maximum-object, one at a time
	if count != 8 {
		t.Errorf("want 8 random monsters (20%% of 40), got %d", count)
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

// Random mobs roam the whole map like the original, not 8 tiles round their
// spawn point, and keep out of the npc-avoidrect areas (Elvine's town).
func TestRandomMobsRoamTheMapOutsideAvoidRects(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(300, 300)
	gm.Name = "elvine"
	e.maps[gm.Name] = gm
	c := mapdata.Configs().Get("elvine")
	if c == nil || len(c.NpcAvoid) == 0 {
		t.Fatal("elvine config should have an npc-avoidrect")
	}
	if !e.spawnRandomMob(gm, npc.NpcTypes[55], 30, 30) {
		t.Fatal("spawn failed")
	}
	var n *npc.NPC
	e.npcs.Range(func(_, v any) bool { n = v.(*npc.NPC); return false })

	far := 0
	for i := 0; i < 300; i++ {
		n.HasWanderDest = false
		if !npcPickWanderDest(n, gm) {
			continue
		}
		if c.InNpcAvoid(n.WanderDestX, n.WanderDestY) {
			t.Fatalf("wander destination (%d,%d) is inside the avoid rect", n.WanderDestX, n.WanderDestY)
		}
		if n.DistanceTo(n.WanderDestX, n.WanderDestY) > 50 {
			far++
		}
	}
	if far == 0 {
		t.Error("random mobs should pick destinations across the map")
	}
}

func countRandomMobs(e *Engine) (random, all int) {
	e.npcs.Range(func(_, v any) bool {
		all++
		if v.(*npc.NPC).RandomMob {
			random++
		}
		return true
	})
	return
}

func TestRandomMobMaxOverridesShare(t *testing.T) {
	useTestConfig(t, "maximum-object = 600\nrandom-mob-generator = 1 1\nrandom-mob-max = 5\n")
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)
	for i := 0; i < 100; i++ {
		e.runRandomMobs()
	}
	if r, _ := countRandomMobs(e); r != 5 {
		t.Errorf("random-mob-max = 5 should cap at 5, got %d", r)
	}
}

func TestRandomMobGroupsMakePacks(t *testing.T) {
	useTestConfig(t, "maximum-object = 600\nrandom-mob-generator = 1 1\nrandom-mob-groups = 1\n")
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)
	most := 0
	for i := 0; i < 50; i++ {
		most = max(most, e.spawnRandomGroup(e.maps["test"], testConfig(t)))
	}
	if most < 2 {
		t.Error("random-mob-groups = 1 should spawn packs")
	}
}

// Spot generator monsters count toward maximum-object: random monsters only
// fill what's left, never crowd the map past it.
func TestRandomMobsStayUnderMaximumObject(t *testing.T) {
	useTestConfig(t, "maximum-object = 20\nrandom-mob-generator = 1 1\nrandom-mob-max = 10\n")
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)
	for i := 0; i < 17; i++ {
		n := npc.NewNPC(int32(1000+i), npc.NpcTypes[10], "test", 20+i, 20)
		e.npcs.Store(n.ObjectID, n)
	}
	guard := npc.NewNPC(2000, npc.NpcTypes[1014], "test", 50, 50) // guards don't count
	e.npcs.Store(guard.ObjectID, guard)
	for i := 0; i < 100; i++ {
		e.runRandomMobs()
	}
	if r, _ := countRandomMobs(e); r != 3 {
		t.Errorf("17 spot monsters of 20: want 3 random monsters, got %d", r)
	}
}

// Lowering the cap (a live map editor change) thins the random monsters out.
func TestRandomMobsThinOutWhenCapDrops(t *testing.T) {
	useTestConfig(t, "maximum-object = 600\nrandom-mob-generator = 1 1\nrandom-mob-max = 10\n")
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)
	for i := 0; i < 50; i++ {
		e.runRandomMobs()
	}
	useTestConfig(t, "maximum-object = 600\nrandom-mob-generator = 1 1\nrandom-mob-max = 4\n")
	for i := 0; i < 50; i++ {
		e.runRandomMobs()
	}
	if r, _ := countRandomMobs(e); r != 4 {
		t.Errorf("cap lowered to 4: want 4 random monsters, got %d", r)
	}
}
