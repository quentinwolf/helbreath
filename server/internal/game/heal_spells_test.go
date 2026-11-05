package game

import (
	"testing"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/player"
)

func healTestPlayer(e *Engine, id int32, name string, side, x, y int) *player.Player {
	p := makeFullTestPlayer(id, name)
	p.Side = side
	p.X, p.Y = x, y
	p.HP = 1
	addPlayerToEngine(e, p)
	return p
}

func TestHealLandsOnTheClickedPlayer(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	p.Side = 2
	ally := healTestPlayer(e, 2, "Ally", 2, 53, 50)
	heal := magic.GetSpellDef(1)

	e.handleHealSpell(p, heal, ally.X, ally.Y, ally.ObjectID, gm)
	if ally.HP <= 1 {
		t.Error("Heal should heal the clicked player")
	}
}

func TestHealFactionRules(t *testing.T) {
	aresden := &player.Player{Side: 1}
	elvine := &player.Player{Side: 2}
	traveller := &player.Player{Side: 0}
	cases := []struct {
		caster, target *player.Player
		want           bool
	}{
		{elvine, elvine, true}, {aresden, aresden, true}, {traveller, traveller, true},
		{traveller, elvine, true}, {aresden, traveller, true},
		{aresden, elvine, false}, {elvine, aresden, false},
	}
	for _, c := range cases {
		if got := canHeal(c.caster, c.target); got != c.want {
			t.Errorf("canHeal(side %d -> side %d) = %v, want %v", c.caster.Side, c.target.Side, got, c.want)
		}
	}

	e, _, p := statusTestSetup(t)
	p.Side = 1
	enemy := healTestPlayer(e, 2, "Enemy", 2, 53, 50)
	if e.healSpellCheck(p, magic.GetSpellDef(1), enemy.X, enemy.Y, enemy.ObjectID) == "" {
		t.Error("healing an enemy citizen should be refused")
	}
}

func TestMassHealHealsAlliesInTheArea(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	p.Side = 1
	near := healTestPlayer(e, 2, "Near", 1, 56, 51)   // inside the 3x3 box around 55,50
	far := healTestPlayer(e, 3, "Far", 1, 58, 50)     // outside it
	enemy := healTestPlayer(e, 4, "Enemy", 2, 54, 49) // inside, but Elvine
	mass := magic.GetSpellDef(68)
	if mass == nil || mass.AreaX != 1 || mass.AreaHealDice.IsZero() {
		t.Fatalf("Mass Heal should have a 3x3 area heal: %+v", mass)
	}

	e.handleHealSpell(p, mass, 55, 50, 0, gm)
	if near.HP <= 1 {
		t.Error("Mass Heal should heal allies in its area")
	}
	if far.HP != 1 || enemy.HP != 1 {
		t.Error("Mass Heal should skip players outside the area and enemies")
	}
}

func TestStaminaRecoveryRestoresStamina(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	p.SP = 0
	hp := p.HP
	e.handleHealSpell(p, magic.GetSpellDef(15), p.X, p.Y, 0, gm)
	if p.SP <= 0 || p.HP != hp {
		t.Errorf("Staminar Recovery should restore SP only (SP %d, HP %d -> %d)", p.SP, hp, p.HP)
	}
}

func TestTravellersCannotHurtEachOther(t *testing.T) {
	a := &player.Player{Side: 0, HP: 10, MapName: "m"}
	b := &player.Player{Side: 0, HP: 10, MapName: "m"}
	c := &player.Player{Side: 1, HP: 10, MapName: "m"}
	if spellCanHitPlayer(a, b) {
		t.Error("a traveller's spell should not hit another traveller")
	}
	if !spellCanHitPlayer(a, c) || !spellCanHitPlayer(c, a) {
		t.Error("travellers and citizens can still fight")
	}
}

func TestDeadPlayersWaitForRestart(t *testing.T) {
	e, _, p := statusTestSetup(t)
	p.HP = 0
	e.handlePlayerDeath(p, 0, "Test", true)
	if p.HP != 0 || p.X != 50 {
		t.Fatal("a dead player should stay down until they click Restart")
	}
	e.handleRespawnRequest(makeClientWithObjectID(p.ObjectID))
	if p.HP <= 0 {
		t.Error("Restart should bring the player back")
	}
}

func TestResurrection(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	p.Side = 1
	dead := healTestPlayer(e, 2, "Fallen", 1, 53, 50)
	dead.HP = 0
	enemy := healTestPlayer(e, 3, "Enemy", 2, 60, 50)
	enemy.HP = 0
	res := magic.GetSpellDef(94)

	if e.healSpellCheck(p, res, 57, 57, 0) == "" {
		t.Error("Resurrection on empty ground should be refused")
	}
	if e.healSpellCheck(p, res, enemy.X, enemy.Y, 0) == "" {
		t.Error("Resurrection on an enemy should be refused")
	}
	if reason := e.healSpellCheck(p, res, dead.X, dead.Y, 0); reason != "" {
		t.Fatalf("Resurrection on a fallen ally refused: %s", reason)
	}

	e.handleHealSpell(p, res, dead.X, dead.Y, 0, gm)
	if dead.HP != 0 || dead.ResurrectBy != p.ObjectID {
		t.Fatal("Resurrection should offer, not raise straight away")
	}
	e.handleResurrectAccept(makeClientWithObjectID(dead.ObjectID))
	if dead.HP != dead.MaxHP/2 || dead.X != 53 || dead.Y != 50 {
		t.Errorf("accepting should stand them up on the spot at half HP (HP %d at %d,%d)", dead.HP, dead.X, dead.Y)
	}
}

func TestHealOnAMonsterReachesThePlayerItFights(t *testing.T) {
	e, gm, p := statusTestSetup(t)
	ally := healTestPlayer(e, 2, "Mauled", 0, 54, 50)
	n := makeTestNPC(14)
	n.MapName = "test"
	n.X, n.Y = 55, 50
	e.npcs.Store(n.ObjectID, n)

	e.handleHealSpell(p, magic.GetSpellDef(1), n.X, n.Y, n.ObjectID, gm)
	if ally.HP <= 1 {
		t.Error("a heal aimed at a monster should reach the player beside it")
	}
}
