package game

import (
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
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

func TestGuardCastsAtDistantMonster(t *testing.T) {
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)

	guard := npc.NewNPC(1, npc.NpcTypes[21], "test", 50, 50)  // Guard: reach 5, has mana
	monster := npc.NewNPC(2, npc.NpcTypes[10], "test", 54, 50) // 4 tiles away
	monster.HP, monster.MaxHP = 100000, 100000
	e.npcs.Store(guard.ObjectID, guard)
	e.npcs.Store(monster.ObjectID, monster)

	now := time.Now()
	for i := 0; i < 20 && monster.HP == monster.MaxHP; i++ {
		now = now.Add(1100 * time.Millisecond)
		e.processNPCTick(guard, now)
	}

	if monster.HP == monster.MaxHP {
		t.Fatal("guard should hit a monster 4 tiles away with magic")
	}
	if guard.X != 50 {
		t.Errorf("guard should cast from where it stands, but moved to x=%d", guard.X)
	}
	if guard.Mana == guard.Type.MaxMana {
		t.Error("casting should cost the guard mana")
	}
}

func TestGuardReturnsToPostAfterAFight(t *testing.T) {
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)

	guard := npc.NewNPC(1, npc.NpcTypes[21], "test", 50, 50)
	guard.X = 60 // chased away from its post
	guard.State = npc.StateChase
	guard.TargetID = 999 // target that no longer exists
	e.npcs.Store(guard.ObjectID, guard)

	now := time.Now()
	for i := 0; i < 60 && guard.DistanceTo(guard.SpawnX, guard.SpawnY) > guard.Type.WanderRange; i++ {
		now = now.Add(1100 * time.Millisecond)
		e.processNPCTick(guard, now)
	}
	if d := guard.DistanceTo(guard.SpawnX, guard.SpawnY); d > guard.Type.WanderRange {
		t.Errorf("guard should walk back to its post, still %d tiles away", d)
	}
}

func TestGuardsLeaveNeutralPlayersAlone(t *testing.T) {
	e := makeTestEngine()
	elvineGuard := npc.NewNPC(1, npc.NpcTypes[1014], "test", 50, 50)
	neutralGuard := npc.NewNPC(2, npc.NpcTypes[21], "test", 50, 50)

	neutral := makeFullTestPlayer(1, "Traveller")
	neutral.Side = npc.SideNeutral // the test helper defaults to Aresden
	elvine := makeFullTestPlayer(2, "Elf")
	elvine.Side = npc.SideElvine
	aresden := makeFullTestPlayer(3, "Enemy")
	aresden.Side = npc.SideAresden
	criminal := makeFullTestPlayer(4, "Crook")
	criminal.Side = npc.SideNeutral
	criminal.PKCount = 3

	cases := []struct {
		guard *npc.NPC
		p     string
		want  bool
	}{
		{elvineGuard, "neutral", false},
		{elvineGuard, "elvine", false},
		{elvineGuard, "aresden", true},
		{elvineGuard, "criminal", true},
		{neutralGuard, "neutral", false},
		{neutralGuard, "aresden", false},
		{neutralGuard, "criminal", true},
	}
	players := map[string]*player.Player{"neutral": neutral, "elvine": elvine, "aresden": aresden, "criminal": criminal}
	for _, c := range cases {
		if got := e.npcCanTarget(c.guard, players[c.p]); got != c.want {
			t.Errorf("%s vs %s player: attack=%v, want %v", c.guard.Type.Name, c.p, got, c.want)
		}
	}
}

func TestSplashSpellsDontHurtFriendlyGuards(t *testing.T) {
	e, p := spellTestSetup(t, spellEnergyBolt)
	p.Side = npc.SideNeutral
	guard := npc.NewNPC(100, npc.NpcTypes[1014], "test", 57, 50)
	e.npcs.Store(guard.ObjectID, guard)
	monster := tankyNPC(e, 101, 56, 50)

	castAt(e, p, spellEnergyBolt, monster.ObjectID, monster.X, monster.Y)

	if monster.HP == monster.MaxHP {
		t.Error("the monster should still be hit")
	}
	if guard.HP != guard.MaxHP || guard.TargetID != 0 {
		t.Error("a guard caught in the splash should be unhurt and not turn on a neutral player")
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

// A guard that chased past its leash used to stand at the leash edge for good:
// each tick it re-targeted the monster beside it, hit the leash and dropped it.
func TestGuardPastLeashWalksHome(t *testing.T) {
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(200, 200)

	guard := npc.NewNPC(1, npc.NpcTypes[1014], "test", 50, 50)
	leash := guard.LeashRange()
	guard.X = 50 + leash + 1
	monster := npc.NewNPC(2, npc.NpcTypes[10], "test", guard.X+1, 50)
	monster.HP, monster.MaxHP = 100000, 100000
	guard.State, guard.TargetID = npc.StateChase, monster.ObjectID
	e.npcs.Store(guard.ObjectID, guard)
	e.npcs.Store(monster.ObjectID, monster)

	startDist := guard.DistanceTo(guard.SpawnX, guard.SpawnY)
	now := time.Now()
	for i := 0; i < 60; i++ {
		now = now.Add(500 * time.Millisecond)
		e.processNPCTick(guard, now)
	}
	if d := guard.DistanceTo(guard.SpawnX, guard.SpawnY); d >= startDist {
		t.Fatalf("guard should walk home after a leash drop, still %d tiles out (was %d)", d, startDist)
	}
}

func TestPassiveMonstersStartNoFights(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm

	p := makeFullTestPlayer(1, "Walker")
	p.Side, p.PKCount = npc.SideElvine, 0 // the guard's own town
	p.X, p.Y = 51, 50
	addPlayerToEngine(e, p)
	gm.AddPlayerToSector(p.X, p.Y, p.ObjectID)

	cat := npc.NewNPC(100, npc.NpcTypes[56], "test", 50, 50)
	guard := npc.NewNPC(101, npc.NpcTypes[1014], "test", 52, 50)
	e.npcs.Store(cat.ObjectID, cat)
	e.npcs.Store(guard.ObjectID, guard)

	if npcHostileToNPC(guard, cat) || npcHostileToNPC(cat, guard) {
		t.Error("guards and cats should leave each other alone")
	}
	if cat.StatusBits() != 0 {
		t.Errorf("a cat should show as Neutral, status %#x", cat.StatusBits())
	}
	now := time.Now()
	for i := 0; i < 20; i++ {
		now = now.Add(500 * time.Millisecond)
		e.processNPCTick(cat, now)
		e.processNPCTick(guard, now)
	}
	if cat.TargetID != 0 || guard.TargetID != 0 {
		t.Errorf("nobody should start a fight: cat targets %d, guard targets %d", cat.TargetID, guard.TargetID)
	}

	// Hit back once attacked (what a player's hit does)
	cat.TargetID, cat.State = p.ObjectID, npc.StateChase
	cat.X, cat.Y = 50, 50
	for i := 0; i < 10 && cat.State != npc.StateAttack; i++ {
		now = now.Add(500 * time.Millisecond)
		e.processNPCTick(cat, now)
	}
	if cat.TargetID != p.ObjectID {
		t.Error("an attacked cat should fight back")
	}
}
