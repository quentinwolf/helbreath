package magic

import "testing"

// These tests pin SpellDB to the original v3.82 Magic.cfg (see
// tools/magicimport): spells at their original IDs, with the original
// server's hit pattern for each DEF_MAGICTYPE.

func TestGetSpellDef(t *testing.T) {
	mm := GetSpellDef(0)
	if mm == nil || mm.Name != "Magic Missile" {
		t.Fatalf("expected Magic Missile at ID 0, got %+v", mm)
	}
	if mm.ManaCost != 8 || mm.Element != ElementLight {
		t.Errorf("Magic Missile: mana=%d element=%d", mm.ManaCost, mm.Element)
	}
	if GetSpellDef(999) != nil {
		t.Error("Expected nil for non-existent spell")
	}
}

func TestSpellDBHasExactlyTheOriginalSpells(t *testing.T) {
	// 90 entries in Magic.cfg minus the two unused Test-Spell placeholders and one gap
	if len(SpellDB) != 87 {
		t.Errorf("Expected the 87 original spells, got %d", len(SpellDB))
	}
	for id, s := range SpellDB {
		if s.ID != id {
			t.Errorf("SpellDB[%d] holds spell ID %d", id, s.ID)
		}
	}
}

func TestOriginalHelbreathSpells(t *testing.T) {
	for id, name := range map[int]string{
		0: "Magic Missile", 1: "Heal", 2: "Create Food", 10: "Energy Bolt",
		11: "Staminar Drain", 12: "Recall", 13: "Defense Shield", 20: "Fire Ball",
		21: "Great Heal", 22: "Tremor", 25: "Hold Person", 27: "Poison",
		30: "Fire Strike", 31: "Summon Creature", 32: "Invisibility", 36: "Cure",
		37: "Lightning Arrow", 40: "Fire Wall", 41: "Fire Field", 43: "Lightning",
		45: "Chill Wind", 46: "Poison Cloud", 47: "Triple Energy Bolt", 50: "Berserk",
		51: "Lightning Bolt", 54: "Spike Field", 55: "Ice Storm", 57: "Ice Strike",
		60: "Energy Strike", 61: "Mass Fire Strike", 64: "Earthworm Strike",
		66: "Armor Break", 70: "Bloody Shock Wave", 72: "Mass Ice Strike",
		74: "Lightning Strike", 81: "Meteor Strike", 83: "Inhibition Casting",
		85: "Blizzard", 91: "Mass Blizzard", 94: "Resurrection", 96: "Earth Shock Wave",
		97: "Explosion", 98: "Medusa Kiss",
	} {
		s := GetSpellDef(id)
		if s == nil {
			t.Errorf("spell %d (%s) missing", id, name)
			continue
		}
		if s.Name != name {
			t.Errorf("spell %d: expected %q, got %q", id, name, s.Name)
		}
	}
}

func TestSpotSpellHitsOnlyTheClickedTile(t *testing.T) {
	mm := GetSpellDef(0) // DAMAGE_SPOT
	if mm.Type != SpellTypeDamage || mm.AreaX != 0 || mm.AreaY != 0 || mm.Line {
		t.Errorf("Magic Missile should be single-tile: %+v", mm)
	}
	if mm.CentreDice != (Dice{Count: 1, Sides: 8}) {
		t.Errorf("Magic Missile dice: %+v", mm.CentreDice)
	}
	if !mm.NeedsTarget() {
		t.Error("single-tile damage spell needs someone on the tile")
	}
}

func TestAreaSpellHasCentreAndSplash(t *testing.T) {
	eb := GetSpellDef(10) // DAMAGE_AREA: "2 2 | 2 4 1 | 2 4 1"
	if eb.Type != SpellTypeAOE || eb.AreaX != 2 || eb.AreaY != 2 {
		t.Errorf("Energy Bolt should splash a 2-tile box: %+v", eb)
	}
	want := Dice{Count: 2, Sides: 4, Bonus: 1}
	if eb.CentreDice != want || eb.AreaDice != want {
		t.Errorf("Energy Bolt dice: centre=%+v area=%+v", eb.CentreDice, eb.AreaDice)
	}
	if eb.NeedsTarget() {
		t.Error("area spells can be cast on empty ground")
	}
}

func TestNoSpotAreaSpellHasNoCentreBonus(t *testing.T) {
	es := GetSpellDef(60) // DAMAGE_AREA_NOSPOT
	if !es.CentreDice.IsZero() || es.AreaDice.IsZero() {
		t.Errorf("Energy Strike should only have area dice: %+v", es)
	}
}

func TestLineSpellsHitALine(t *testing.T) {
	for _, id := range []int{51, 70, 85, 96} { // Lightning Bolt, Bloody Shock Wave, Blizzard, Earth Shock Wave
		s := GetSpellDef(id)
		if !s.Line || s.LineDice.IsZero() {
			t.Errorf("%s should hit along a line: %+v", s.Name, s)
		}
	}
}

func TestIceSpellsFreeze(t *testing.T) {
	is := GetSpellDef(57) // ICE: box damage with g1, freeze for g3 seconds
	if is.AreaDice.IsZero() || is.ApplyEffect != EffectIce || is.Duration != 4 {
		t.Errorf("Ice Strike should damage its box and freeze for 4s: %+v", is)
	}
}

func TestOffensiveSpellsReachTheWholeScreen(t *testing.T) {
	for _, id := range []int{0, 10, 25, 81} {
		if s := GetSpellDef(id); s.Range != MaxCastRange {
			t.Errorf("%s range: expected %d, got %d", s.Name, MaxCastRange, s.Range)
		}
	}
	if heal := GetSpellDef(1); heal.Range != 0 {
		t.Errorf("Heal is self-cast, got range %d", heal.Range)
	}
}

func TestLearningNeedsOnlyINT(t *testing.T) {
	for _, s := range SpellDB {
		if s.ReqLevel != 0 || s.ReqMAG != 0 {
			t.Errorf("%s: Magic.cfg only requires INT, got level=%d MAG=%d", s.Name, s.ReqLevel, s.ReqMAG)
		}
	}
	if fb := GetSpellDef(20); fb.ReqINT != 26 {
		t.Errorf("Fire Ball ReqINT: expected 26, got %d", fb.ReqINT)
	}
}

func TestCooldownsFollowTheDelayColumn(t *testing.T) {
	if eb := GetSpellDef(10); eb.Cooldown != eb.CastTime {
		t.Errorf("Energy Bolt has no Delay: cooldown should equal cast time, got %d vs %d", eb.Cooldown, eb.CastTime)
	}
	if res := GetSpellDef(94); res.Cooldown != 600_000 {
		t.Errorf("Resurrection Delay is 600s, got %dms", res.Cooldown)
	}
}

func TestStatusEffectSpells(t *testing.T) {
	tests := []struct {
		id     int
		effect EffectType
		level  int
	}{
		{13, EffectDefenseShield, 1},   // Defense Shield
		{44, EffectDefenseShield, 2},   // Great Defense Shield
		{33, EffectMagicProtection, 1}, // Protection From Magic
		{65, EffectMagicProtection, 3}, // Absolute Magic Protection
		{25, EffectIce, 1},             // Hold Person
		{27, EffectPoison, 1},          // Poison
		{32, EffectInvisibility, 1},    // Invisibility
		{50, EffectBerserk, 1},         // Berserk
		{83, EffectInhibition, 1},      // Inhibition Casting
	}
	for _, tt := range tests {
		s := GetSpellDef(tt.id)
		if s.ApplyEffect != tt.effect || s.EffectLevel != tt.level {
			t.Errorf("%s: expected effect %d level %d, got %d level %d",
				s.Name, tt.effect, tt.level, s.ApplyEffect, s.EffectLevel)
		}
	}
}

func TestPoisonSpellHasTickDamage(t *testing.T) {
	p := GetSpellDef(27)
	if p.TickDamage <= 0 || p.TickIntervalMs != 3000 {
		t.Errorf("Poison should tick: damage=%d interval=%d", p.TickDamage, p.TickIntervalMs)
	}
}

func TestDiceRange(t *testing.T) {
	d := Dice{Count: 2, Sides: 4, Bonus: 1}
	if d.Min() != 3 || d.Max() != 9 {
		t.Errorf("2d4+1: expected 3-9, got %d-%d", d.Min(), d.Max())
	}
	if !(Dice{}).IsZero() {
		t.Error("empty dice should be zero")
	}
}
