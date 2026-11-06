package game

import (
	"testing"

	"github.com/juanrossi/hbonline/server/internal/items"
	"github.com/juanrossi/hbonline/server/internal/magic"
)

const (
	spellCreateFood    = 2
	spellDefenseShield = 13
)

func TestDefenseShieldOnYourOwnSide(t *testing.T) {
	e, p := spellTestSetup(t, spellDefenseShield)
	p.AdminLevel, p.Side = 0, 1
	ally := makeFullTestPlayer(2, "Ally")
	ally.Side, ally.X, ally.Y = 1, 53, 50
	addPlayerToEngine(e, ally)

	castAt(e, p, spellDefenseShield, ally.ObjectID, ally.X, ally.Y)

	if !ally.Effects.HasEffect(magic.EffectDefenseShield) {
		t.Error("Defense Shield should go on the ally you click")
	}
	if p.Effects.HasEffect(magic.EffectDefenseShield) {
		t.Error("...not on the caster")
	}
}

func TestDefenseShieldNotOnTheOtherSide(t *testing.T) {
	e, p := spellTestSetup(t, spellDefenseShield)
	p.AdminLevel, p.Side = 0, 1
	enemy := makeFullTestPlayer(2, "Enemy")
	enemy.Side, enemy.X, enemy.Y = 2, 53, 50
	addPlayerToEngine(e, enemy)

	castAt(e, p, spellDefenseShield, enemy.ObjectID, enemy.X, enemy.Y)
	if enemy.Effects.HasEffect(magic.EffectDefenseShield) {
		t.Error("a player can't shield the other town")
	}

	p.AdminLevel = 1
	clear(p.Cooldowns)
	castAt(e, p, spellDefenseShield, enemy.ObjectID, enemy.X, enemy.Y)
	if !enemy.Effects.HasEffect(magic.EffectDefenseShield) {
		t.Error("a GM can shield anyone")
	}
}

func TestDefenseShieldOnEmptyGroundIsSelfCast(t *testing.T) {
	e, p := spellTestSetup(t, spellDefenseShield)
	castAt(e, p, spellDefenseShield, 0, 55, 55)
	if !p.Effects.HasEffect(magic.EffectDefenseShield) {
		t.Error("with nobody clicked the caster shields themselves")
	}
}

func TestCreateFoodWhereYouClick(t *testing.T) {
	e, p := spellTestSetup(t, spellCreateFood)
	castAt(e, p, spellCreateFood, 0, 56, 47)
	var got *items.GroundItem
	e.groundItems.Range(func(_, v any) bool { got = v.(*items.GroundItem); return false })
	if got == nil || got.X != 56 || got.Y != 47 || (got.Item.DefID != itemMeat && got.Item.DefID != itemBaguette) {
		t.Fatalf("Create Food should leave a Meat or Baguette at (56,47), got %+v", got)
	}

	mp := p.MP
	castAt(e, p, spellCreateFood, 0, 50+magic.MaxCastRange+1, 50)
	if p.MP != mp {
		t.Error("Create Food off screen should be refused at no cost")
	}
}
