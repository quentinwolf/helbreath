package game

import (
	"log"
	"math/rand"
	"time"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
)

// Lasting spell fields: Fire Wall, Fire Field, Poison Cloud, Cloud Kill, Ice
// Storm, Spike Field (the original's dynamic objects, DEF_MAGICTYPE_CREATE_DYNAMIC).
// A cast does no damage itself; it leaves field tiles, and once a second
// (DynamicObjectEffectProcessor in the original server) each tile hurts whoever
// stands on it or next to it. Someone touching several tiles is hit by each.
// The client draws the fields itself from the spell effect (30 s).

// fieldFreeMaps are the towns where the original refused field spells outside
// crusades.
var fieldFreeMaps = map[string]bool{"aresden": true, "elvine": true, "arefarm": true, "elvfarm": true}

// magicField is one burning (or poisoned, frozen, spiked) tile.
type magicField struct {
	kind     magic.FieldKind
	mapName  string
	x, y     int
	casterID int32
	power    int // poison strength (Poison Cloud 18, Cloud Kill 40)
	expires  time.Time
}

// castFieldSpell lays a field spell's tiles on the map.
func (e *Engine) castFieldSpell(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	e.broadcastSpellEffect(p, spell, 0, p.X, p.Y, tx, ty, 0, 0, false, gm)

	expires := time.Now().Add(time.Duration(spell.Duration) * time.Second)
	e.fieldsMu.Lock()
	defer e.fieldsMu.Unlock()
	placed := 0
	for _, t := range spell.FieldTiles(p.X, p.Y, tx, ty) {
		x, y := t[0], t[1]
		if x < 0 || y < 0 || x >= gm.Width || y >= gm.Height {
			continue
		}
		// Fire needs ground it can burn on; and one field per tile, the first stays
		if spell.Field.Kind == magic.FieldFire && !gm.Tiles[y][x].Walkable {
			continue
		}
		if e.fieldAtLocked(p.MapName, x, y) != nil {
			continue
		}
		e.fields = append(e.fields, &magicField{
			kind: spell.Field.Kind, mapName: p.MapName, x: x, y: y,
			casterID: p.ObjectID, power: spell.Field.Power, expires: expires,
		})
		placed++
	}
	log.Printf("Spell %s at (%d,%d): %d field tiles for %ds", spell.Name, tx, ty, placed, spell.Duration)
}

// fieldAtLocked is the live field on a tile, if any. Hold fieldsMu.
func (e *Engine) fieldAtLocked(mapName string, x, y int) *magicField {
	for _, f := range e.fields {
		if f.x == x && f.y == y && f.mapName == mapName {
			return f
		}
	}
	return nil
}

// fieldReach is how far from its tile a field hurts (1 = the 3x3 around it).
func fieldReach(kind magic.FieldKind) int {
	switch kind {
	case magic.FieldIceStorm:
		return 2
	case magic.FieldSpike:
		return -1 // only when stepped on (fieldStepOn)
	}
	return 1
}

// processFields drops expired fields and lets the rest do their damage.
func (e *Engine) processFields(now time.Time) {
	e.fieldsMu.Lock()
	live := e.fields[:0]
	for _, f := range e.fields {
		if now.Before(f.expires) {
			live = append(live, f)
		}
	}
	clear(e.fields[len(live):])
	e.fields = live
	fields := append([]*magicField(nil), live...)
	e.fieldsMu.Unlock()
	if len(fields) == 0 {
		return
	}

	// Who stands where, only on maps with a field
	active := map[string]bool{}
	for _, f := range fields {
		active[f.mapName] = true
	}
	type tileKey struct {
		m    string
		x, y int
	}
	npcsAt := map[tileKey][]*npc.NPC{}
	e.npcs.Range(func(_, v any) bool {
		if n := v.(*npc.NPC); n.IsAlive() && active[n.MapName] {
			k := tileKey{n.MapName, n.X, n.Y}
			npcsAt[k] = append(npcsAt[k], n)
		}
		return true
	})
	playersAt := map[tileKey][]*player.Player{}
	e.players.Range(func(_, v any) bool {
		if t := v.(*player.Player); t.HP > 0 && active[t.MapName] {
			k := tileKey{t.MapName, t.X, t.Y}
			playersAt[k] = append(playersAt[k], t)
		}
		return true
	})

	for _, f := range fields {
		reach := fieldReach(f.kind)
		if reach < 0 {
			continue
		}
		// Damage is the caster's: their side rules, kill credit and exp. A field
		// whose caster left the map (or the game) stops hurting anyone.
		caster := e.getPlayerByID(f.casterID)
		gm, ok := e.maps[f.mapName]
		if caster == nil || caster.MapName != f.mapName || !ok {
			continue
		}
		for y := f.y - reach; y <= f.y+reach; y++ {
			for x := f.x - reach; x <= f.x+reach; x++ {
				k := tileKey{f.mapName, x, y}
				for _, n := range npcsAt[k] {
					if n.IsAlive() && e.playerCanHurtNPC(caster, n) {
						e.spellHurtNPC(caster, n, fieldDamage(f, caster), gm)
					}
				}
				for _, t := range playersAt[k] {
					if t.HP > 0 && t.MapName == f.mapName && t.X == x && t.Y == y && spellCanHitPlayer(caster, t) {
						e.fieldHurtPlayer(f, caster, t, gm)
					}
				}
			}
		}
	}
}

// fieldDamage is one tick of a field's damage (the original's dice).
func fieldDamage(f *magicField, caster *player.Player) int {
	var dmg int
	switch f.kind {
	case magic.FieldFire:
		dmg = 1 + rand.Intn(6) // 1d6
	case magic.FieldPoisonCloud:
		if f.power < 20 {
			dmg = 1 + rand.Intn(6) // 1d6
		} else {
			dmg = 1 + rand.Intn(8) // 1d8
		}
	case magic.FieldIceStorm:
		dmg = 3 + rand.Intn(3) + rand.Intn(3) + rand.Intn(3) + 5 // 3d3+5
	case magic.FieldSpike:
		dmg = 2 + rand.Intn(4) + rand.Intn(4) // 2d4
	}
	if caster.GodMode {
		dmg *= 1000
	}
	return dmg
}

// fieldHurtPlayer is a field's tick on a player: damage, then Poison Cloud
// poisons (unless resisted) and Ice Storm freezes for 20 s, as the original.
func (e *Engine) fieldHurtPlayer(f *magicField, caster, t *player.Player, gm *mapdata.GameMap) {
	var freeze time.Duration
	if f.kind == magic.FieldIceStorm && t.Effects != nil && !t.Effects.HasEffect(magic.EffectIce) {
		freeze = 20 * time.Second
	}
	e.spellHurtPlayer(caster, t, fieldDamage(f, caster), freeze, 1, gm)
	if f.kind == magic.FieldPoisonCloud && t.HP > 0 && t.Effects != nil && !t.Effects.HasEffect(magic.EffectPoison) &&
		!magic.CheckMagicResistance(caster.EffectiveMAG(), t.EffectiveINT(), 1) {
		now := time.Now()
		t.Effects.AddEffect(&magic.ActiveEffect{
			Type:         magic.EffectPoison,
			Level:        1,
			ExpiresAt:    now.Add(30 * time.Second),
			SourceID:     caster.ObjectID,
			TickDamage:   (f.power + 1) / 2, // the original rolls 1d(strength) per poison tick
			TickInterval: 3 * time.Second,
			LastTick:     now,
		})
		e.sendNotification(t, "You have been poisoned", 2)
	}
}

// fieldStepOn hurts a player who walks onto a Spike Field tile (2d4), the only
// time spikes hurt in the original (monsters walk over them).
func (e *Engine) fieldStepOn(t *player.Player, gm *mapdata.GameMap) {
	e.fieldsMu.Lock()
	f := e.fieldAtLocked(t.MapName, t.X, t.Y)
	e.fieldsMu.Unlock()
	if f == nil || f.kind != magic.FieldSpike || !time.Now().Before(f.expires) {
		return
	}
	caster := e.getPlayerByID(f.casterID)
	if caster == nil || caster.MapName != f.mapName || !spellCanHitPlayer(caster, t) {
		return
	}
	e.spellHurtPlayer(caster, t, fieldDamage(f, caster), 0, 0, gm)
}
