package game

import (
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/npc"
)

func TestGuardsAndMonstersAreHostile(t *testing.T) {
	guard := npc.NewNPC(1, npc.NpcTypes[1014], "test", 50, 50) // Guard-Elvine
	monster := npc.NewNPC(2, npc.NpcTypes[10], "test", 51, 50) // Slime
	monster2 := npc.NewNPC(3, npc.NpcTypes[10], "test", 52, 50)
	shop := npc.NewNPC(4, npc.NpcTypes[15], "test", 53, 50) // ShopKeeper

	if !npcHostileToNPC(guard, monster) || !npcHostileToNPC(monster, guard) {
		t.Error("guards and monsters should fight each other")
	}
	if npcHostileToNPC(monster, monster2) {
		t.Error("monsters should not fight each other")
	}
	if npcHostileToNPC(monster, shop) || npcHostileToNPC(guard, shop) {
		t.Error("shopkeepers stay out of fights")
	}
}

func TestGuardFightsAdjacentMonster(t *testing.T) {
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)

	guard := npc.NewNPC(1, npc.NpcTypes[1014], "test", 50, 50)
	monster := npc.NewNPC(2, npc.NpcTypes[10], "test", 51, 50)
	monster.HP, monster.MaxHP = 100000, 100000 // survive long enough to observe
	e.npcs.Store(guard.ObjectID, guard)
	e.npcs.Store(monster.ObjectID, monster)

	now := time.Now()
	for i := 0; i < 40 && monster.HP == monster.MaxHP; i++ {
		now = now.Add(500 * time.Millisecond)
		e.processNPCTick(guard, now)
	}

	if guard.TargetID != monster.ObjectID {
		t.Fatalf("guard should target the monster, targets %d", guard.TargetID)
	}
	if monster.HP == monster.MaxHP {
		t.Error("guard should have hit the monster")
	}
}

func TestMonsterKilledByGuardRespawnsWithoutLoot(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm

	guard := npc.NewNPC(1, npc.NpcTypes[1014], "test", 50, 50)
	monster := npc.NewNPC(2, npc.NpcTypes[10], "test", 51, 50)
	monster.HP = 1
	e.npcs.Store(guard.ObjectID, guard)
	e.npcs.Store(monster.ObjectID, monster)

	for i := 0; i < 50 && monster.IsAlive(); i++ {
		e.npcAttackNPC(guard, monster, gm)
	}
	if monster.IsAlive() {
		t.Fatal("monster with 1 HP should die to a guard")
	}
	if guard.TargetID != 0 {
		t.Error("guard should drop its target once the monster is dead")
	}
	if _, still := e.npcs.Load(monster.ObjectID); !still {
		t.Error("a normal monster stays registered so it can respawn")
	}
}
