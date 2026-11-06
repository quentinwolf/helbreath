package game

import (
	"strings"
	"testing"

	"github.com/juanrossi/hbonline/server/internal/items"
	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/player"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

const (
	spellStaminarDrain   = 11
	spellHoldPerson      = 25
	spellPossession      = 26
	spellParalyze        = 35
	spellConfuseLanguage = 42
	spellMassConfusion   = 71
	spellMassIllusion    = 90
)

// debuffSetup: an Aresden mage at (50,50) and an Elvine victim at (54,50).
// Nobody resists unless the test says so.
func debuffSetup(t *testing.T, spellID int) (*Engine, *player.Player, *player.Player) {
	t.Helper()
	e, p := spellTestSetup(t, spellID)
	p.Side = 1
	v := makeFullTestPlayer(2, "Victim")
	v.Side, v.X, v.Y = 2, 54, 50
	addPlayerToEngine(e, v)
	old := resistRoll
	resistRoll = func(int, int, int) bool { return false }
	t.Cleanup(func() { resistRoll = old })
	return e, p, v
}

func tryWalk(e *Engine, p *player.Player, dir int32) {
	e.handleMotion(makeClientWithObjectID(p.ObjectID), &pb.MotionRequest{Direction: dir, Action: 1})
}

func TestHoldPersonStopsEveryDirection(t *testing.T) {
	for _, spell := range []int{spellHoldPerson, spellParalyze} {
		e, p, v := debuffSetup(t, spell)
		castAt(e, p, spell, v.ObjectID, v.X, v.Y)
		if !v.Effects.HasEffect(magic.EffectHold) {
			t.Fatalf("spell %d: victim should be held", spell)
		}
		for dir := int32(1); dir <= 8; dir++ {
			v.LastMoveTime = v.LastMoveTime.Add(-10e9)
			tryWalk(e, v, dir)
			if v.X != 54 || v.Y != 50 {
				t.Fatalf("spell %d: a held player walked direction %d", spell, dir)
			}
		}
	}
}

func TestSpellDamageBreaksHold(t *testing.T) {
	e, p, v := debuffSetup(t, spellHoldPerson)
	castAt(e, p, spellHoldPerson, v.ObjectID, v.X, v.Y)
	e.spellHurtPlayer(p, v, 1, 0, 0, e.maps["test"])
	if v.Effects.HasEffect(magic.EffectHold) {
		t.Error("spell damage should break Hold Person")
	}
	tryWalk(e, v, 3)
	if v.X != 55 {
		t.Error("once the hold breaks the player can walk again")
	}
}

func TestHoldPersonHoldsMonsters(t *testing.T) {
	e, p, _ := debuffSetup(t, spellHoldPerson)
	n := tankyNPC(e, 100, 52, 52)
	castAt(e, p, spellHoldPerson, n.ObjectID, n.X, n.Y)
	e.moveNPC(n, e.maps["test"], 53, 52, 3)
	if n.X != 52 {
		t.Error("a held monster should not move")
	}
}

func TestHoldPersonNotOnOwnSide(t *testing.T) {
	e, p, v := debuffSetup(t, spellHoldPerson)
	v.Side = p.Side
	castAt(e, p, spellHoldPerson, v.ObjectID, v.X, v.Y)
	if v.Effects.HasEffect(magic.EffectHold) {
		t.Error("Hold Person should not work on your own side")
	}
}

func TestStaminarDrain(t *testing.T) {
	e, p, v := debuffSetup(t, spellStaminarDrain)
	v.SP, v.MaxSP = 200, 200
	near := makeFullTestPlayer(3, "Near")
	near.Side, near.X, near.Y, near.SP, near.MaxSP = 2, 56, 52, 200, 200 // inside the 3-tile box
	addPlayerToEngine(e, near)

	castAt(e, p, spellStaminarDrain, v.ObjectID, v.X, v.Y)

	// Clicked player: 4d6+20 + 2d6+15 = 41..71
	if d := 200 - v.SP; d < 41 || d > 71 {
		t.Errorf("clicked player should lose 41-71 stamina, lost %d", d)
	}
	// Box: 2d6+15 = 17..27
	if d := 200 - near.SP; d < 17 || d > 27 {
		t.Errorf("player in the box should lose 17-27 stamina, lost %d", d)
	}
}

func TestMassConfusionHitsTheBox(t *testing.T) {
	e, p, v := debuffSetup(t, spellMassConfusion)
	far := makeFullTestPlayer(3, "Far")
	far.Side, far.X, far.Y = 2, 58, 50 // 4 tiles from the click: outside the 3-tile box
	addPlayerToEngine(e, far)

	castAt(e, p, spellMassConfusion, 0, 54, 51) // empty tile next to the victim

	if v.Effects.GetEffectLevel(magic.EffectConfusion) != magic.ConfuseAllegiance {
		t.Error("victim in the box should be confused")
	}
	if far.Effects.HasEffect(magic.EffectConfusion) {
		t.Error("player outside the box should not be confused")
	}
}

func TestIllusionRemembersTheCaster(t *testing.T) {
	e, p, v := debuffSetup(t, spellMassIllusion)
	castAt(e, p, spellMassIllusion, 0, v.X, v.Y)
	eff := v.Effects.GetEffect(magic.EffectConfusion)
	if eff == nil || eff.Level != magic.ConfuseIllusion || eff.SourceID != p.ObjectID {
		t.Fatalf("Mass Illusion should leave an illusion of the caster, got %+v", eff)
	}
}

func TestConfuseLanguageScramblesTheTargetsChat(t *testing.T) {
	e, p, v := debuffSetup(t, spellConfuseLanguage)
	castAt(e, p, spellConfuseLanguage, v.ObjectID, v.X, v.Y)
	scrambled := 0
	for i := 0; i < 60; i++ {
		if out := confusedSpeech(v, "hello there friend"); out != "hello there friend" {
			scrambled++
			if strings.ContainsAny(out, "abcdefghijklmnopqrstuvwxyz") {
				t.Fatalf("scrambled speech should be gibberish, got %q", out)
			}
		}
	}
	if scrambled < 20 || scrambled > 58 {
		t.Errorf("about 2 messages in 3 should be scrambled, got %d of 60", scrambled)
	}

	v.AdminLevel = 1
	if confusedSpeech(v, "gm talking") != "gm talking" {
		t.Error("a GM's chat is never scrambled")
	}
}

func TestPossessionTakesAnItemFromAfar(t *testing.T) {
	e, p, _ := debuffSetup(t, spellPossession)
	def := items.GetItemDef(91) // RedPotion
	e.groundItems.Store(int32(900), &items.GroundItem{GroundID: 900, Item: items.NewItem(def, 3), MapName: "test", X: 57, Y: 55})

	castAt(e, p, spellPossession, 0, 57, 55)

	if _, still := e.groundItems.Load(int32(900)); still {
		t.Fatal("Possession should lift the item off the ground")
	}
	found := false
	for i := 0; i < 50; i++ {
		if it := p.Inventory.GetItem(i); it != nil && it.DefID == 91 {
			found = true
		}
	}
	if !found {
		t.Error("the item should be in the caster's bag")
	}
}
