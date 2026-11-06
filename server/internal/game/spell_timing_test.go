package game

import (
	"testing"
	"time"
)

func TestSpellHitsWhenTheEffectLands(t *testing.T) {
	if d := spellImpactDelay(81, 10, 10, 15, 10); d != 450*time.Millisecond {
		t.Errorf("Meteor Strike should land after 450 ms, got %v", d)
	}
	near, far := spellImpactDelay(20, 0, 0, 2, 0), spellImpactDelay(20, 0, 0, 8, 0)
	if near <= 0 || far <= near {
		t.Errorf("a Fire Ball takes longer to reach a farther target: %v vs %v", near, far)
	}
	if spellImpactDelay(43, 0, 0, 5, 5) != 0 {
		t.Error("lightning hits at once")
	}

	e := makeTestEngine()
	e.delayed.instant = false
	hit := false
	e.after(450*time.Millisecond, func() { hit = true })
	e.runDelayed(time.Now().Add(400 * time.Millisecond))
	if hit {
		t.Fatal("hit before its time")
	}
	e.runDelayed(time.Now().Add(500 * time.Millisecond))
	if !hit {
		t.Fatal("not hit once its time came")
	}
}
