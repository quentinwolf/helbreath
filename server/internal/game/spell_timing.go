package game

import (
	"math"
	"sync"
	"time"
)

// Spell damage lands when the spell's effect visibly hits, not the moment it's
// cast: the client draws a meteor falling for about half a second, a fireball
// flying to its target, and monsters dying before that looked wrong. (The
// original client hid this by sending the cast only after the casting motion.)
// The timings follow the client's effects (client/src/game/spells/SpellEffects.ts).

// spellImpactDelay is how long after the cast a damage spell's effect hits.
func spellImpactDelay(spellID, fromX, fromY, toX, toY int) time.Duration {
	dist := math.Hypot(float64(toX-fromX)*32, float64(toY-fromY)*32) // pixels
	travel := func(pxPerMs float64) time.Duration {
		return time.Duration(dist/pxPerMs) * time.Millisecond
	}
	var d time.Duration
	switch spellID {
	case 0, 37, 56: // Magic Missile, Lightning Arrow, Mass Lightning Arrow: 50 px per 20 ms
		d = travel(2.5)
	case 10, 47, 20, 30, 61, 6: // Energy Bolt, Triple Energy Bolt, Fire Ball, Fire Strike, Mass Fire Strike (x2): 1500 px/s
		d = travel(1.5)
	case 60: // Energy Strike: 2000 px/s
		d = travel(2.0)
	case 81: // Meteor Strike: falls from 300 px right / 600 px up at 1500 px/s
		d = 450 * time.Millisecond
	case 57, 72: // Ice Strike, Mass Ice Strike: shards fade in for 7 frames of 20 ms, then drop
		d = 350 * time.Millisecond
	case 45, 63: // Chill Wind, Mass Chill Wind: 200 px drop at 1500 px/s
		d = 130 * time.Millisecond
	case 85, 91: // Blizzard, Mass Blizzard: 300 ms out, first shards land 200 ms later
		d = 500 * time.Millisecond
	case 70, 96: // Bloody Shock Wave, Earth Shock Wave: 400 ms waves
		d = 400 * time.Millisecond
	}
	return min(d, 1500*time.Millisecond)
}

// delayedActions are actions waiting for their time (spell hits).
type delayedActions struct {
	mu      sync.Mutex
	pending []delayedAction
	instant bool // run everything at once (tests)
}

type delayedAction struct {
	at time.Time
	fn func()
}

// after runs fn on the engine tick once d has passed (at once if d <= 0).
func (e *Engine) after(d time.Duration, fn func()) {
	if d <= 0 || e.delayed.instant {
		fn()
		return
	}
	e.delayed.mu.Lock()
	e.delayed.pending = append(e.delayed.pending, delayedAction{at: time.Now().Add(d), fn: fn})
	e.delayed.mu.Unlock()
}

// runDelayed runs the actions that are due (processTick, every 100 ms).
func (e *Engine) runDelayed(now time.Time) {
	e.delayed.mu.Lock()
	var due []func()
	keep := e.delayed.pending[:0]
	for _, a := range e.delayed.pending {
		if now.Before(a.at) {
			keep = append(keep, a)
		} else {
			due = append(due, a.fn)
		}
	}
	e.delayed.pending = keep
	e.delayed.mu.Unlock()
	for _, fn := range due {
		fn()
	}
}
