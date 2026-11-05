package game

import (
	"testing"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// Original spell IDs from Magic.cfg used below.
const (
	spellMagicMissile  = 0  // DAMAGE_SPOT: the clicked tile only
	spellEnergyBolt    = 10 // DAMAGE_AREA: tile + 2-tile box around it
	spellLightningBolt = 51 // DAMAGE_LINEAR: line from the caster + box
)

// spellTestSetup makes a map with a mage at (50,50) who knows the given spell.
func spellTestSetup(t *testing.T, spellID int) (*Engine, *player.Player) {
	t.Helper()
	e := makeTestEngine()
	gm := makeTestGameMap(120, 120)
	e.maps["test"] = gm

	p := makeFullTestPlayer(1, "Mage")
	p.MP, p.MaxMP = 1000, 1000
	p.INT, p.MAG = 200, 200
	p.LearnedSpells = map[int]bool{spellID: true}
	addPlayerToEngine(e, p)
	if magic.GetSpellDef(spellID) == nil {
		t.Fatalf("spell %d missing from SpellDB", spellID)
	}
	return e, p
}

// tankyNPC places a monster with lots of HP so a hit never kills it.
func tankyNPC(e *Engine, id int32, x, y int) *npc.NPC {
	n := npc.NewNPC(id, npc.NpcTypes[10], "test", x, y)
	n.HP, n.MaxHP = 100000, 100000
	e.npcs.Store(n.ObjectID, n)
	return n
}

func castAt(e *Engine, p *player.Player, spellID int, targetID int32, x, y int) {
	e.handleSpellCast(makeClientWithObjectID(p.ObjectID), &pb.SpellCastRequest{
		SpellId:        int32(spellID),
		TargetId:       targetID,
		TargetPosition: &pb.Vec2{X: int32(x), Y: int32(y)},
	})
}

func TestOriginalSpellsAtOriginalIDs(t *testing.T) {
	for id, name := range map[int]string{
		0: "Magic Missile", 1: "Heal", 10: "Energy Bolt", 12: "Recall",
		20: "Fire Ball", 30: "Fire Strike", 32: "Invisibility", 40: "Fire Wall",
		50: "Berserk", 57: "Ice Strike", 81: "Meteor Strike",
	} {
		s := magic.GetSpellDef(id)
		if s == nil || s.Name != name {
			t.Errorf("spell %d: want %q, got %+v", id, name, s)
		}
	}
}

func TestAreaSpellHitsEveryoneInItsBox(t *testing.T) {
	e, p := spellTestSetup(t, spellEnergyBolt)
	centre := tankyNPC(e, 100, 56, 50)
	inBox := tankyNPC(e, 101, 58, 52) // 2 tiles off diagonally: inside the 2x2 box
	outside := tankyNPC(e, 102, 59, 50)

	castAt(e, p, spellEnergyBolt, 0, 56, 50)

	if centre.HP == centre.MaxHP {
		t.Error("monster on the clicked tile should be hit")
	}
	if inBox.HP == inBox.MaxHP {
		t.Error("monster 2 tiles away should be hit by the splash")
	}
	if outside.HP != outside.MaxHP {
		t.Error("monster 3 tiles away is outside the box and should be untouched")
	}
}

func TestAreaSpellCanBeCastOnEmptyGround(t *testing.T) {
	e, p := spellTestSetup(t, spellEnergyBolt)
	castAt(e, p, spellEnergyBolt, 0, 55, 55)
	if p.MP == p.MaxMP {
		t.Error("area spell on empty ground should still be cast (and cost mana)")
	}
}

func TestSpotSpellOnEmptyTileCostsNothing(t *testing.T) {
	e, p := spellTestSetup(t, spellMagicMissile)
	castAt(e, p, spellMagicMissile, 0, 55, 55)
	if p.MP != p.MaxMP {
		t.Error("single-tile spell with nobody there should not cost mana")
	}
}

func TestSpellsReachAcrossTheScreen(t *testing.T) {
	e, p := spellTestSetup(t, spellMagicMissile)
	far := tankyNPC(e, 100, 50+magic.MaxCastRange, 50)
	tooFar := tankyNPC(e, 101, 50+magic.MaxCastRange+1, 50)

	castAt(e, p, spellMagicMissile, far.ObjectID, far.X, far.Y)
	if far.HP == far.MaxHP {
		t.Errorf("monster %d tiles away should be in range", magic.MaxCastRange)
	}

	mp := p.MP
	castAt(e, p, spellMagicMissile, tooFar.ObjectID, tooFar.X, tooFar.Y)
	if tooFar.HP != tooFar.MaxHP || p.MP != mp {
		t.Error("monster beyond MaxCastRange should be out of range, at no cost")
	}
}

func TestNamedTargetWinsOverClickedTile(t *testing.T) {
	e, p := spellTestSetup(t, spellMagicMissile)
	n := tankyNPC(e, 100, 54, 50)
	castAt(e, p, spellMagicMissile, n.ObjectID, 53, 51) // click next to it
	if n.HP == n.MaxHP {
		t.Error("spell should land on the named target's tile")
	}
}

func TestLineSpellHitsAlongThePath(t *testing.T) {
	e, p := spellTestSetup(t, spellLightningBolt)
	onLine := tankyNPC(e, 100, 54, 50)
	offLine := tankyNPC(e, 101, 54, 54)

	castAt(e, p, spellLightningBolt, 0, 58, 50)

	if onLine.HP == onLine.MaxHP {
		t.Error("monster between caster and target should be hit by the line")
	}
	if offLine.HP != offLine.MaxHP {
		t.Error("monster well off the line and outside the box should be untouched")
	}
}

func TestSpellsFollowFactionRules(t *testing.T) {
	e, p := spellTestSetup(t, spellEnergyBolt)
	p.Side = 1 // Aresden

	enemy := makeFullTestPlayer(2, "Elvine")
	enemy.Side = 2
	enemy.X, enemy.Y = 56, 50
	addPlayerToEngine(e, enemy)

	ally := makeFullTestPlayer(3, "Ally")
	ally.Side = 1
	ally.X, ally.Y = 57, 51
	addPlayerToEngine(e, ally)

	castAt(e, p, spellEnergyBolt, 0, 56, 50)

	if enemy.HP == enemy.MaxHP {
		t.Error("enemy-faction player in the blast should be hit")
	}
	if ally.HP != ally.MaxHP {
		t.Error("same-faction player in the blast should not be hit")
	}
	if p.HP != p.MaxHP {
		t.Error("caster should never hit themselves")
	}
}

func TestSpotSpellOnAllyIsRefusedForFree(t *testing.T) {
	e, p := spellTestSetup(t, spellMagicMissile)
	p.Side = 1
	ally := makeFullTestPlayer(2, "Ally")
	ally.Side = 1
	ally.X, ally.Y = 53, 50
	addPlayerToEngine(e, ally)

	castAt(e, p, spellMagicMissile, ally.ObjectID, ally.X, ally.Y)

	if ally.HP != ally.MaxHP || p.MP != p.MaxMP {
		t.Error("casting at a same-faction player should be refused without cost")
	}
}
