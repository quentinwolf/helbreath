package game

import (
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/magic"
)

const (
	spellFireWall   = 40
	spellFireField  = 41
	spellSpikeField = 54
	spellCloudKill  = 73
)

func TestFieldSpellsComeFromMagicCfg(t *testing.T) {
	for id, want := range map[int]magic.FieldSpec{
		spellFireWall:   {Kind: magic.FieldFire, Layout: magic.FieldWall, Size: 2, Power: 8},
		spellFireField:  {Kind: magic.FieldFire, Layout: magic.FieldSquare, Size: 1, Power: 8},
		46:              {Kind: magic.FieldPoisonCloud, Layout: magic.FieldSquare, Size: 1, Power: 18},
		spellSpikeField: {Kind: magic.FieldSpike, Layout: magic.FieldSquare, Size: 2, Power: 8},
		spellCloudKill:  {Kind: magic.FieldPoisonCloud, Layout: magic.FieldSquare, Size: 3, Power: 40},
	} {
		s := magic.GetSpellDef(id)
		if s.Field != want || s.DealsDamage() {
			t.Errorf("%s: field %+v (damage on cast %v), want %+v and no hit on cast", s.Name, s.Field, s.DealsDamage(), want)
		}
	}
}

func TestFireWallRunsAcrossTheCast(t *testing.T) {
	fw := magic.GetSpellDef(spellFireWall)
	cases := []struct {
		name   string
		tx, ty int
		want   [][2]int
	}{
		{"cast east: wall runs north-south", 55, 50, [][2]int{{55, 50}, {55, 51}, {55, 49}, {55, 52}, {55, 48}}},
		{"cast north: wall runs east-west", 50, 45, [][2]int{{50, 45}, {51, 45}, {49, 45}, {52, 45}, {48, 45}}},
		{"cast south-east: wall runs south-west to north-east", 53, 53, [][2]int{{53, 53}, {52, 54}, {54, 52}, {51, 55}, {55, 51}}},
	}
	for _, c := range cases {
		got := fw.FieldTiles(50, 50, c.tx, c.ty)
		if len(got) != len(c.want) {
			t.Fatalf("%s: %v", c.name, got)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

func TestFireWallBurnsMonstersEverySecond(t *testing.T) {
	e, p := spellTestSetup(t, spellFireWall)
	beside := tankyNPC(e, 100, 56, 50) // next to the wall (wall is x=55, y 48..52)
	away := tankyNPC(e, 101, 58, 50)

	castAt(e, p, spellFireWall, 0, 55, 50)
	if beside.HP != beside.MaxHP {
		t.Fatal("the cast itself should do no damage, the fire does")
	}
	if p.MP == p.MaxMP {
		t.Fatal("Fire Wall on empty ground should be cast")
	}

	now := time.Now()
	e.processFields(now)
	first := beside.MaxHP - beside.HP
	// 3 wall tiles (y 49, 50, 51) touch (56,50): 3 x 1d6
	if first < 3 || first > 18 {
		t.Errorf("one tick next to 3 fire tiles should do 3d6, did %d", first)
	}
	e.processFields(now.Add(time.Second))
	if beside.MaxHP-beside.HP <= first {
		t.Error("fire should keep burning on the next tick")
	}
	if away.HP != away.MaxHP {
		t.Error("a monster 3 tiles from the wall should not burn")
	}
	if beside.TargetID != p.ObjectID {
		t.Error("a burning monster should turn on the caster")
	}

	// Gone after its 30 s
	hp := beside.HP
	e.processFields(now.Add(31 * time.Second))
	if beside.HP != hp {
		t.Error("the wall should be out after 30 s")
	}
}

func TestFieldTilesDoNotStack(t *testing.T) {
	e, p := spellTestSetup(t, spellFireField)
	n := tankyNPC(e, 100, 55, 50)
	castAt(e, p, spellFireField, 0, 55, 50)
	castAt(e, p, spellFireField, 0, 55, 50)
	if len(e.fields) != 9 {
		t.Fatalf("Fire Field twice on the same spot should still be 9 tiles, got %d", len(e.fields))
	}
	e.processFields(time.Now())
	// In the middle of a 3x3: all 9 tiles reach it, 9d6
	if d := n.MaxHP - n.HP; d < 9 || d > 54 {
		t.Errorf("monster in the middle of Fire Field should take 9d6, took %d", d)
	}
}

func TestSpikeFieldHurtsOnlyWhenSteppedOn(t *testing.T) {
	e, p := spellTestSetup(t, spellSpikeField)
	n := tankyNPC(e, 100, 55, 50)
	castAt(e, p, spellSpikeField, 0, 55, 50)
	e.processFields(time.Now())
	if n.HP != n.MaxHP {
		t.Error("spikes don't hurt monsters standing in them (original: players stepping in only)")
	}
}

func TestNoFieldsInTown(t *testing.T) {
	e, p := spellTestSetup(t, spellFireWall)
	e.maps["aresden"] = e.maps["test"]
	p.MapName = "aresden"
	castAt(e, p, spellFireWall, 0, 55, 50)
	if len(e.fields) != 0 || p.MP != p.MaxMP {
		t.Error("field spells are refused in town, at no cost")
	}
}
