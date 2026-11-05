package game

import (
	"log"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// Heal spells, by their Magic.cfg type: Heal, Great Heal, Critical Heal and
// Regenerate (HPUP_SPOT) heal whoever is on the clicked tile; Mass Heal does
// that and heals everyone in the box around it; the Staminar Recovery spells
// (SPUP_AREA) do the same for stamina; Cure removes poison.
const (
	cfgTypeHealSpot  = 2
	cfgTypeStaminaUp = 7
	cfgTypeCure      = 17
	cfgTypeMassHeal  = 27
	cfgTypeResurrect = 32
)

// notifyResurrectOffer is the Notification type that asks a dead player to
// accept a Resurrection; its message is the caster's name.
const notifyResurrectOffer = 11

// canHeal is who may heal whom. The original let anyone heal anyone; we keep
// Aresden and Elvine from healing each other. Travellers can heal, and be
// healed by, anybody.
func canHeal(caster, target *player.Player) bool {
	return caster.Side == 0 || target.Side == 0 || caster.Side == target.Side
}

// healSpellCheck refuses a heal before mana is spent: out of range, or aimed
// at an enemy citizen.
func (e *Engine) healSpellCheck(p *player.Player, spell *magic.SpellDef, tx, ty int, targetID int32) string {
	if max(abs(tx-p.X), abs(ty-p.Y)) > magic.MaxCastRange {
		return "Target out of range"
	}
	if spell.CfgType == cfgTypeResurrect {
		t := e.deadPlayerAt(p.MapName, tx, ty)
		switch {
		case t == nil:
			return "There is nobody here to resurrect"
		case !canResurrect(p, t):
			return "You cannot resurrect an enemy"
		}
		return ""
	}
	if t := e.getPlayerByID(targetID); t != nil && !canHeal(p, t) && spell.AreaX == 0 && spell.AreaY == 0 {
		return "You cannot heal an enemy"
	}
	return ""
}

// handleHealSpell heals (or restores stamina to, or cures) the player on the
// clicked tile, or the caster when nobody they can heal is there, plus
// everyone they can heal in the spell's area.
func (e *Engine) handleHealSpell(p *player.Player, spell *magic.SpellDef, tx, ty int, targetID int32, gm *mapdata.GameMap) {
	if spell.CfgType == cfgTypeResurrect {
		e.castResurrection(p, spell, tx, ty, gm)
		return
	}
	centre := p
	if t := e.getPlayerByID(targetID); t != nil && t.HP > 0 && t.MapName == p.MapName && canHeal(p, t) {
		centre = t
	} else if val, isNPC := e.npcs.Load(targetID); isNPC {
		if n := val.(*npc.NPC); n.MasterID == p.ObjectID {
			e.healSummon(p, spell, n, gm)
			return
		}
		// Clicked a monster: heal the player it's fighting beside, if any
		if t := e.healablePlayerNear(p, tx, ty); t != nil {
			centre, targetID = t, t.ObjectID
		}
	}
	if targetID == 0 && (tx != p.X || ty != p.Y) && (spell.AreaX > 0 || spell.AreaY > 0) {
		centre = nil // an area heal aimed at open ground
	}
	if centre != nil {
		tx, ty = centre.X, centre.Y
	}

	shown := 0
	if centre != nil {
		shown = e.applyHeal(p, spell, centre, rollDice(spell.HealDice))
	}
	if spell.AreaX > 0 || spell.AreaY > 0 {
		e.players.Range(func(_, value any) bool {
			t := value.(*player.Player)
			if t.HP > 0 && t.MapName == p.MapName && canHeal(p, t) &&
				abs(t.X-tx) <= spell.AreaX && abs(t.Y-ty) <= spell.AreaY {
				e.applyHeal(p, spell, t, rollDice(spell.AreaHealDice))
			}
			return true
		})
	}

	var shownID int32
	if centre != nil {
		shownID = centre.ObjectID
	}
	e.broadcastSpellEffect(p, spell, shownID, p.X, p.Y, tx, ty, 0, shown, false, gm)
}

// applyHeal gives one player the spell's effect and returns the amount shown.
// The caster's MAG adds a third of itself to HP heals, as before.
func (e *Engine) applyHeal(caster *player.Player, spell *magic.SpellDef, t *player.Player, roll int) int {
	switch spell.CfgType {
	case cfgTypeCure:
		if t.Effects != nil && t.Effects.HasEffect(magic.EffectPoison) {
			t.Effects.RemoveEffect(magic.EffectPoison)
			if t != caster {
				e.sendNotification(t, caster.Name+" cured your poison", 0)
			}
		}
		return 0
	case cfgTypeStaminaUp:
		gain := roll
		if gain <= 0 {
			gain = spell.HealAmount
		}
		t.SP = min(t.MaxSP, t.SP+gain)
		e.sendStatUpdate(t)
		return 0
	}
	if roll <= 0 {
		roll = spell.HealAmount
	}
	heal := roll + caster.EffectiveMAG()/3
	t.HP = min(t.MaxHP, t.HP+heal)
	e.sendStatUpdate(t)
	if t != caster {
		log.Printf("Player %s healed %s for %d HP", caster.Name, t.Name, heal)
	}
	return heal
}

// healSummon heals the caster's own summoned creature.
func (e *Engine) healSummon(p *player.Player, spell *magic.SpellDef, n *npc.NPC, gm *mapdata.GameMap) {
	if !n.IsAlive() || spell.CfgType == cfgTypeCure || spell.CfgType == cfgTypeStaminaUp {
		return
	}
	heal := max(rollDice(spell.HealDice), spell.HealAmount) + p.EffectiveMAG()/3
	n.HP = min(n.MaxHP, n.HP+heal)
	e.broadcastSpellEffect(p, spell, n.ObjectID, p.X, p.Y, n.X, n.Y, 0, heal, false, gm)
}

// canResurrect: as the original, only your own side (GMs can raise anyone).
func canResurrect(caster, target *player.Player) bool {
	return caster.AdminLevel >= 1 || sameSide(caster, target)
}

// deadPlayerAt finds a fallen player lying on (or next to) a tile, nearest first.
func (e *Engine) deadPlayerAt(mapName string, x, y int) *player.Player {
	var found *player.Player
	best := 2
	e.players.Range(func(_, value any) bool {
		t := value.(*player.Player)
		if t.HP <= 0 && t.MapName == mapName {
			if d := max(abs(t.X-x), abs(t.Y-y)); d < best {
				found, best = t, d
			}
		}
		return true
	})
	return found
}

// castResurrection offers to raise a fallen player. Like the original, they're
// asked first; accepting (handleResurrectAccept) stands them up where they fell.
func (e *Engine) castResurrection(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	t := e.deadPlayerAt(p.MapName, tx, ty)
	if t == nil || !canResurrect(p, t) {
		return
	}
	t.ResurrectBy = p.ObjectID
	e.broadcastSpellEffect(p, spell, t.ObjectID, p.X, p.Y, t.X, t.Y, 0, 0, false, gm)
	e.sendNotification(t, p.Name, notifyResurrectOffer)
	e.sendNotification(p, "You offer to resurrect "+t.Name, 0)
}

// handleResurrectAccept stands a dead player up on the spot at half HP.
func (e *Engine) handleResurrectAccept(client *network.Client) {
	p := e.getPlayerByClient(client)
	if p == nil || p.HP > 0 || p.ResurrectBy == 0 {
		return
	}
	gm, ok := e.maps[p.MapName]
	if !ok {
		return
	}
	if caster := e.getPlayerByID(p.ResurrectBy); caster != nil {
		e.sendNotification(caster, p.Name+" has been resurrected", 0)
	}
	p.ResurrectBy = 0
	p.HP = max(1, p.MaxHP/2)

	data, _ := network.Encode(network.MsgRespawnEvent, &pb.RespawnEvent{
		Position:  &pb.Vec2{X: int32(p.X), Y: int32(p.Y)},
		Direction: int32(p.Direction),
		MapName:   p.MapName,
		Hp:        int32(p.HP),
		Mp:        int32(p.MP),
		Sp:        int32(p.SP),
	})
	p.Send(data)
	e.broadcastToNearby(gm, p.X, p.Y, p.ObjectID, network.MsgPlayerAppear, p.ToPlayerAppear())
	e.sendStatUpdate(p)
	log.Printf("Player %s was resurrected", p.Name)
}

// healablePlayerNear finds the nearest living player the caster may heal on or
// next to a tile (not the caster).
func (e *Engine) healablePlayerNear(p *player.Player, x, y int) *player.Player {
	var found *player.Player
	best := 2
	e.players.Range(func(_, value any) bool {
		t := value.(*player.Player)
		if t != p && t.HP > 0 && t.MapName == p.MapName && canHeal(p, t) {
			if d := max(abs(t.X-x), abs(t.Y-y)); d < best {
				found, best = t, d
			}
		}
		return true
	})
	return found
}
