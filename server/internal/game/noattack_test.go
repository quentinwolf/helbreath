package game

import (
	"testing"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
)

// useTestConfig makes "test" a map with a no-attack area at 10,10-20,20.
func useTestConfig(t *testing.T, text string) {
	t.Helper()
	c, errs := mapdata.ParseMapConfig("test", []byte(text))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	mapdata.SetConfigs(mapdata.MapConfigs{"test": c})
	t.Cleanup(func() { mapdata.SetConfigs(mapdata.BuiltinConfigs()) })
}

func TestNoAttackAreaStopsPvPAndSpells(t *testing.T) {
	useTestConfig(t, "no-attack-area = 1 10 10 20 20\n")
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	attacker := makeFullTestPlayer(1, "Attacker")
	attacker.Side, attacker.DEX = 1, 200
	target := makeFullTestPlayer(2, "Target")
	target.Side = 2
	addPlayerToEngine(e, attacker)
	addPlayerToEngine(e, target)

	// Target inside, attacker outside: no melee damage
	attacker.X, attacker.Y = 9, 15
	target.X, target.Y = 10, 15
	for i := 0; i < 20; i++ {
		e.handlePvPAttack(attacker, target)
	}
	if target.HP != target.MaxHP {
		t.Errorf("a player in a no-attack area shouldn't take melee damage (HP %d/%d)", target.HP, target.MaxHP)
	}
	// Spells don't hurt them either
	e.spellHurtPlayer(attacker, target, 10, 0, 0, gm)
	if target.HP != target.MaxHP {
		t.Error("a player in a no-attack area shouldn't take spell damage")
	}
	// Outside, spells hurt
	target.X = 30
	e.spellHurtPlayer(attacker, target, 10, 0, 0, gm)
	if target.HP != target.MaxHP-10 {
		t.Errorf("outside the area spells should hurt: HP %d/%d", target.HP, target.MaxHP)
	}

	// No attack spells cast from inside
	var attack *magic.SpellDef
	for id := 0; id < 100 && attack == nil; id++ {
		if s := magic.GetSpellDef(id); s != nil && s.DealsDamage() && s.Range > 0 {
			attack = s
		}
	}
	if attack == nil {
		t.Skip("no attack spell")
	}
	attacker.X, attacker.Y = 15, 15
	const safe = "You can't cast attack spells in a safe area"
	if reason := e.validateSpellTarget(attacker, attack, 16, 15, 0); reason != safe {
		t.Errorf("attack spells shouldn't be cast from a no-attack area, got %q", reason)
	}
	attacker.X = 30
	if reason := e.validateSpellTarget(attacker, attack, 31, 15, 0); reason == safe {
		t.Error("outside the area attack spells are allowed")
	}
}

func TestWholeMapNoAttack(t *testing.T) {
	useTestConfig(t, "no-attack-area = 1 -10 -10 -10 -10\n")
	if !mapdata.IsNoAttackTile("test", 0, 0) || !mapdata.IsNoAttackTile("TEST", 55, 70) {
		t.Error("a -10 rectangle makes the whole map a no-attack area, as in the original")
	}
	useTestConfig(t, "no-attack-area = 1 0 0 5 5\n")
	if mapdata.IsNoAttackTile("test", 1, 1) {
		t.Error("the original only counts rectangles with a top above 0")
	}
}

// testConfig is the "test" map's config set by useTestConfig.
func testConfig(t *testing.T) *mapdata.MapConfig {
	t.Helper()
	c := mapdata.Configs().Get("test")
	if c == nil {
		t.Fatal("no test config")
	}
	return c
}
