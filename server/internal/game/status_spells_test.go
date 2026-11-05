package game

import (
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
)

func TestBeingHitKeepsInvisibility(t *testing.T) {
	// The original reveals you only when you attack or cast, not when you're hit.
	p := makeTestPlayer()
	n := makeTestNPC(11)
	p.Effects.AddEffect(&magic.ActiveEffect{
		Type: magic.EffectInvisibility, Level: 1, ExpiresAt: time.Now().Add(time.Minute),
	})
	for i := 0; i < 20; i++ {
		p.HP = p.MaxHP
		NPCAttackPlayer(n, p)
	}
	if !p.Effects.HasEffect(magic.EffectInvisibility) {
		t.Error("being hit should not break invisibility")
	}
}

func TestBreakInvisibility(t *testing.T) {
	e := makeTestEngine()
	e.maps["test"] = makeTestGameMap(100, 100)
	p := makeFullTestPlayer(1, "Hider")
	addPlayerToEngine(e, p)
	p.Effects.AddEffect(&magic.ActiveEffect{
		Type: magic.EffectInvisibility, Level: 1, ExpiresAt: time.Now().Add(time.Minute),
	})
	if p.ToPlayerAppear().Status&magic.StatusInvisible == 0 {
		t.Fatal("PlayerAppear should carry the invisible status bit")
	}
	e.breakInvisibility(p)
	if p.Effects.HasEffect(magic.EffectInvisibility) || p.ToPlayerAppear().Status != 0 {
		t.Error("breakInvisibility should clear the effect and the status bit")
	}
}

func TestDetectInvisibilityRadius(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	caster := makeFullTestPlayer(1, "Seer")
	addPlayerToEngine(e, caster)

	invisible := func(id int32, x, y int) func() bool {
		p := makeFullTestPlayer(id, "Hider")
		p.X, p.Y = x, y
		p.Effects.AddEffect(&magic.ActiveEffect{
			Type: magic.EffectInvisibility, Level: 1, ExpiresAt: time.Now().Add(time.Minute),
		})
		addPlayerToEngine(e, p)
		return func() bool { return p.Effects.HasEffect(magic.EffectInvisibility) }
	}
	near := invisible(2, 58, 50) // 8 tiles from the target: revealed
	far := invisible(3, 59, 50)  // 9 tiles: stays hidden

	detect := magic.GetSpellDef(34)
	if detect == nil || !isDetectInvisibility(detect) {
		t.Fatal("spell 34 should be Detect Invisibility (and not apply invisibility)")
	}
	e.castDetectInvisibility(caster, detect, 50, 50, gm)
	if near() {
		t.Error("player 8 tiles away should be revealed")
	}
	if !far() {
		t.Error("player 9 tiles away should stay invisible")
	}
}

func TestSummonRoll(t *testing.T) {
	if got := summonRoll(1); got != "Slime" {
		t.Errorf("level 1: got %s, want Slime", got)
	}
	// Level 180: one of the three strongest, mostly the strongest
	allowed := map[string]int{"Gagoyle": 0, "Demon": 0, "MasterMage-Orc": 0}
	for i := 0; i < 600; i++ {
		got := summonRoll(180)
		if _, ok := allowed[got]; !ok {
			t.Fatalf("level 180 rolled %s", got)
		}
		allowed[got]++
	}
	if allowed["Gagoyle"] < allowed["Demon"] || allowed["Demon"] < allowed["MasterMage-Orc"] {
		t.Errorf("rolls should favour the strongest creature: %v", allowed)
	}
	// Level 60: Troll at best, never stronger
	for i := 0; i < 200; i++ {
		if got := summonRoll(60); got != "Troll" && got != "Hellbound" && got != "Orc-Mage" {
			t.Fatalf("level 60 rolled %s", got)
		}
	}
	for i, c := range summonCreatures {
		if npc.NpcTypeByName(c.Name) == nil {
			t.Errorf("summon table creature %q has no NPC type", c.Name)
		}
		if i > 0 && c.Level < summonCreatures[i-1].Level {
			t.Errorf("summon table out of level order at %s", c.Name)
		}
	}
}

func TestSummonOnTargetTile(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	summon := magic.GetSpellDef(31)
	if reason := e.validateSpellTarget(p, summon, p.X+8, p.Y, 0); reason != "" {
		t.Fatalf("summoning 8 tiles away refused: %s", reason)
	}
	if e.validateSpellTarget(p, summon, p.X+9, p.Y, 0) == "" {
		t.Error("summoning 9 tiles away should be out of range")
	}
	e.castSummon(p, summon, p.X+6, p.Y+3, gm)
	var n *npc.NPC
	e.npcs.Range(func(_, v any) bool { n = v.(*npc.NPC); return false })
	if n == nil || n.X != p.X+6 || n.Y != p.Y+3 {
		t.Errorf("summon should appear on the clicked tile, got %+v", n)
	}
}

func TestMonstersIgnorePolymorphedPlayers(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	n := makeTestNPC(14)
	n.MapName = "test"
	n.X, n.Y = p.X+2, p.Y
	e.npcs.Store(n.ObjectID, n)
	gm.AddPlayerToSector(p.X, p.Y, p.ObjectID)
	if found, _ := e.findNearestPlayer(n, gm); found != p {
		t.Fatal("test setup: the monster should see the player")
	}
	e.castPolymorph(p, magic.GetSpellDef(52), 0, gm)
	if found, _ := e.findNearestPlayer(n, gm); found != nil {
		t.Error("monsters should not go after a polymorphed player")
	}
}

func TestSummonsFightWildMonsters(t *testing.T) {
	wild := makeTestNPC(14) // Orc
	summon := makeTestNPC(14)
	summon.MasterID = 99
	if !npcHostileToNPC(summon, wild) || !npcHostileToNPC(wild, summon) {
		t.Error("a summoned creature and a wild monster should fight each other")
	}
	other := makeTestNPC(10)
	if npcHostileToNPC(wild, other) {
		t.Error("wild monsters should not fight each other")
	}
}

func TestNPCPolymorphSprite(t *testing.T) {
	n := makeTestNPC(14)
	if n.ShownSpriteType() == magic.PolymorphSpriteType {
		t.Skip("test NPC is already a zombie")
	}
	n.PolymorphUntil = time.Now().Add(time.Minute)
	if n.ToNpcAppear().NpcType != magic.PolymorphSpriteType {
		t.Error("a polymorphed NPC should appear as a Zombie")
	}
	n.PolymorphUntil = time.Now().Add(-time.Second)
	if n.ToNpcAppear().NpcType == magic.PolymorphSpriteType {
		t.Error("polymorph should wear off")
	}
}

func statusTestSetup(t *testing.T) (*Engine, *mapdata.GameMap, *player.Player) {
	t.Helper()
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	p := makeFullTestPlayer(1, "Mage")
	addPlayerToEngine(e, p)
	return e, gm, p
}

func TestSummonCreatureFollowsAndExpires(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	summon := magic.GetSpellDef(31)
	e.castSummon(p, summon, p.X, p.Y, gm)
	if e.summonsOf(p.ObjectID) != 1 {
		t.Fatal("Summon Creature should create one follower")
	}
	// Magery 0 allows one at a time
	e.castSummon(p, summon, p.X, p.Y, gm)
	if e.summonsOf(p.ObjectID) != 1 {
		t.Fatal("a second summon should be refused below 40 Magery")
	}

	var n *npc.NPC
	e.npcs.Range(func(_, v any) bool { n = v.(*npc.NPC); return false })
	if n.MasterID != p.ObjectID || !n.Summoned {
		t.Fatal("summon should belong to its caster")
	}

	// Master walks off: the summon follows
	p.X, p.Y = 70, 50
	start := n.DistanceTo(p.X, p.Y)
	now := time.Now()
	for i := 0; i < 10; i++ {
		now = now.Add(2 * time.Second)
		e.processSummonTick(n, now, gm)
	}
	if n.DistanceTo(p.X, p.Y) >= start {
		t.Errorf("summon should follow its master (was %d away, now %d)", start, n.DistanceTo(p.X, p.Y))
	}

	// Time up: it vanishes
	e.processSummonTick(n, n.SummonExpires.Add(time.Second), gm)
	if e.summonsOf(p.ObjectID) != 0 {
		t.Error("summon should vanish after its lifetime")
	}
}

func TestInvisibilityTargetsSelfOrOther(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	friend := makeFullTestPlayer(2, "Friend")
	friend.X = 52
	addPlayerToEngine(e, friend)
	invis := magic.GetSpellDef(32)

	e.castInvisibility(p, invis, 0, gm)
	if !p.Effects.HasEffect(magic.EffectInvisibility) {
		t.Error("casting on your own tile should make you invisible")
	}
	e.castInvisibility(p, invis, friend.ObjectID, gm)
	if !friend.Effects.HasEffect(magic.EffectInvisibility) {
		t.Error("casting on another player should make them invisible")
	}
}

func TestTravellersCanHideButNotRecall(t *testing.T) {
	_, _, p := statusTestSetup(t)
	p.Side = 0
	if statusSpellCheck(p, magic.GetSpellDef(12), p.X, p.Y) == "" {
		t.Error("a traveller has no town to Recall to")
	}
	for _, id := range []int{32, 34} {
		if reason := statusSpellCheck(p, magic.GetSpellDef(id), p.X, p.Y); reason != "" {
			t.Errorf("spell %d refused for a traveller: %s", id, reason)
		}
	}
	p.Side = 1
	for _, id := range []int{12, 32, 34} {
		if reason := statusSpellCheck(p, magic.GetSpellDef(id), p.X, p.Y); reason != "" {
			t.Errorf("spell %d refused for a citizen: %s", id, reason)
		}
	}
}

func TestRecallTakesCitizenHome(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	town := makeTestGameMap(300, 300)
	town.Name = "aresden"
	e.maps["aresden"] = town
	p.Side = 1
	e.castRecall(p, magic.GetSpellDef(12), gm)
	if p.MapName != "aresden" || p.X != 170 || p.Y != 146 {
		t.Errorf("Recall should land in Aresden at 170,146, got %s %d,%d", p.MapName, p.X, p.Y)
	}
}

func TestPolymorphMonster(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	n := makeTestNPC(14)
	n.MapName = "test"
	e.npcs.Store(n.ObjectID, n)
	e.castPolymorph(p, magic.GetSpellDef(52), n.ObjectID, gm)
	if n.ToNpcAppear().NpcType != magic.PolymorphSpriteType {
		t.Error("Polymorph should turn the monster into a Zombie")
	}
	e.castPolymorph(p, magic.GetSpellDef(52), 0, gm)
	if p.ToPlayerAppear().Status&magic.StatusPolymorph == 0 {
		t.Error("Polymorph on your own tile should change you")
	}
}
