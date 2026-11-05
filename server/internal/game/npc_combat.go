package game

import (
	"math/rand"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// npcStopMoving tells clients an NPC has stopped, so they switch it from the
// walk animation back to standing. Without it a stopped NPC walks in place.
func (e *Engine) npcStopMoving(n *npc.NPC, gm *mapdata.GameMap) {
	if n.Action == 0 {
		return
	}
	n.Action = 0
	e.broadcastToNearby(gm, n.X, n.Y, -1, network.MsgNpcMotion, &pb.NpcMotion{
		ObjectId:  n.ObjectID,
		Action:    0,
		Direction: int32(n.Direction),
		Position:  &pb.Vec2{X: int32(n.X), Y: int32(n.Y)},
		Speed:     int32(n.Type.MoveSpeed),
		Name:      n.Type.Name,
		NpcType:   int32(n.ShownSpriteType()),
	})
}

// npcDropTarget ends a fight: the NPC stops, then idles if it's at its post or
// wanders back home if a chase took it away.
func (e *Engine) npcDropTarget(n *npc.NPC, gm *mapdata.GameMap) {
	n.TargetID = 0
	e.npcStopMoving(n, gm)
	if !n.InHomeArea(n.X, n.Y) {
		n.State = npc.StateWander
	} else {
		n.State = npc.StateIdle
	}
}

// npcSpellChoices are the attack spells NPC casters use, weakest first. A
// caster uses the strongest one its INT allows and its mana covers.
var npcSpellChoices = []int{0, 10, 20, 43} // Magic Missile, Energy Bolt, Fire Ball, Lightning

// npcPickSpell returns the spell an NPC would cast now, or nil (no mana, not a caster).
func npcPickSpell(n *npc.NPC) *magic.SpellDef {
	if n.Type.MaxMana <= 0 {
		return nil
	}
	// Casting skill: INT, or the NPC's magic hit ratio for types without INT data
	// (the city guards only have the latter)
	skill := max(n.Type.INT, n.Type.MagicHitRatio)
	var best *magic.SpellDef
	for _, id := range npcSpellChoices {
		s := magic.GetSpellDef(id)
		if s != nil && s.ReqINT <= skill && s.ManaCost <= n.Mana {
			best = s
		}
	}
	return best
}

// npcReach is how far an NPC can attack from right now: its ranged reach if it
// shoots (no mana needed) or has mana for a spell, otherwise melee only.
func npcReach(n *npc.NPC) int {
	if n.Type.AttackRange <= 1 {
		return 1
	}
	if n.Type.MaxMana > 0 && npcPickSpell(n) == nil {
		return 1 // a caster out of mana has to close in
	}
	return n.ReachRange()
}

// npcRegenMana slowly refills an NPC caster's mana (about 10% per second).
func npcRegenMana(n *npc.NPC) {
	if n.Type.MaxMana > 0 && n.Mana < n.Type.MaxMana {
		n.Mana = min(n.Type.MaxMana, n.Mana+max(1, n.Type.MaxMana/50))
	}
}

// npcCastSpell casts an attack spell from an NPC at its current target.
// Only the target is hit (no splash), so guards don't scorch bystanders.
func (e *Engine) npcCastSpell(n *npc.NPC, spell *magic.SpellDef, gm *mapdata.GameMap) {
	tx, ty, ok := e.npcTargetPosition(n)
	if !ok {
		return
	}
	n.Mana -= spell.ManaCost
	n.Direction = n.DirectionTo(tx, ty)

	e.broadcastToNearby(gm, n.X, n.Y, -1, network.MsgMotionEvent, &pb.MotionEvent{
		ObjectId:  n.ObjectID,
		OwnerType: 2,
		Action:    3, // attack animation (monster sprites have no cast pose)
		Direction: int32(n.Direction),
		Position:  &pb.Vec2{X: int32(n.X), Y: int32(n.Y)},
		Name:      n.Type.Name,
	})
	e.broadcastToNearby(gm, n.X, n.Y, -1, network.MsgSpellEffect, &pb.SpellEffectEvent{
		CasterId:       n.ObjectID,
		SpellId:        int32(spell.ID),
		TargetId:       n.TargetID,
		CasterPosition: &pb.Vec2{X: int32(n.X), Y: int32(n.Y)},
		TargetPosition: &pb.Vec2{X: int32(tx), Y: int32(ty)},
		SpriteId:       int32(spell.SpriteID),
		SoundId:        int32(spell.SoundID),
	})

	dmg := rollDice(spell.CentreDice) + rollDice(spell.AreaDice) + n.Type.INT/10
	dmg = applyElementBonus(dmg, spell.Element)

	if val, isNPC := e.npcs.Load(n.TargetID); isNPC {
		victim := val.(*npc.NPC)
		dmg = max(1, dmg-min(victim.Type.Defense/2, dmg*60/100))
		killed := victim.TakeDamage(dmg)
		e.broadcastToNearby(gm, victim.X, victim.Y, -1, network.MsgDamageEvent, &pb.DamageEvent{
			AttackerId: n.ObjectID, TargetId: victim.ObjectID, TargetType: 2,
			Damage: int32(dmg), TargetHp: int32(victim.HP), TargetMaxHp: int32(victim.MaxHP),
		})
		if !killed && victim.TargetID == 0 && npcHostileToNPC(victim, n) {
			victim.TargetID = n.ObjectID
			victim.State = npc.StateChase
		}
		if killed {
			e.handleNPCKilledByNPC(victim, n, gm)
		}
		return
	}

	target := e.getPlayerByID(n.TargetID)
	if target == nil {
		return
	}
	if target.GodMode {
		dmg = 0
	} else if target.Effects != nil && target.Effects.HasEffect(magic.EffectMagicProtection) {
		if target.Effects.GetEffectLevel(magic.EffectMagicProtection) >= 3 {
			dmg = 0
		} else {
			dmg /= 2
		}
	}
	target.HP -= dmg
	killed := target.HP <= 0 && dmg > 0
	if target.HP < 0 {
		target.HP = 0
	}
	e.broadcastToNearby(gm, target.X, target.Y, -1, network.MsgDamageEvent, &pb.DamageEvent{
		AttackerId: n.ObjectID, TargetId: target.ObjectID, TargetType: 1,
		Damage: int32(dmg), Miss: dmg == 0, TargetHp: int32(target.HP), TargetMaxHp: int32(target.MaxHP),
	})
	e.sendStatUpdate(target)
	if dmg > 0 {
		e.cancelLogoutOnDamage(target)
	}
	if killed {
		e.handlePlayerDeath(target, n.ObjectID, n.Type.Name, true)
	}
}

// NPC-vs-NPC combat: town guards fight monsters, and monsters fight back.
// Shopkeepers and other non-combat NPCs stay out of it, and monsters don't
// fight each other.

// isGuardType reports whether an NPC type is a fighting non-monster (a guard).
func isGuardType(t *npc.NpcType) bool {
	return t.Side != npc.SideMonster && t.AggroRange > 0 && !npc.IsShopNPC(t.ID)
}

// playerCanHurtNPC reports whether a player's attacks and spells affect an
// NPC. Shopkeepers never take damage, and a guard that wouldn't fight you
// (your town's, or a neutral guard while you're neutral and not a criminal)
// can't be hurt by you either, so splash damage and misclicks don't start a
// fight with the town watch.
func (e *Engine) playerCanHurtNPC(p *player.Player, n *npc.NPC) bool {
	if npc.IsShopNPC(n.Type.ID) || n.MasterID == p.ObjectID {
		return false
	}
	if isGuardType(n.Type) && !e.npcCanTarget(n, p) {
		return false
	}
	return true
}

// npcHostileToNPC reports whether a would attack b.
func npcHostileToNPC(a, b *npc.NPC) bool {
	if a == b {
		return false
	}
	// A summoned creature fights for its master: it and wild monsters are enemies.
	wildA := a.Type.Side == npc.SideMonster && a.MasterID == 0
	wildB := b.Type.Side == npc.SideMonster && b.MasterID == 0
	if (a.MasterID != 0 && wildB) || (wildA && b.MasterID != 0) {
		return true
	}
	return (wildA && isGuardType(b.Type)) || (isGuardType(a.Type) && wildB)
}

// findNearestEnemyNPC returns the closest living NPC that n would attack.
func (e *Engine) findNearestEnemyNPC(n *npc.NPC, maxDist int) (*npc.NPC, int) {
	if n.Type.Side != npc.SideMonster && !isGuardType(n.Type) {
		return nil, 999
	}
	if e.npcFighterMaps != nil && n.MasterID == 0 && !isGuardType(n.Type) && !e.npcFighterMaps[n.MapName] {
		return nil, 999 // a wild monster with no guard or summon on its map to fight
	}
	var nearest *npc.NPC
	nearestDist := 999
	e.npcs.Range(func(_, value any) bool {
		other := value.(*npc.NPC)
		if other.MapName != n.MapName || !other.IsAlive() || !npcHostileToNPC(n, other) {
			return true
		}
		if d := n.DistanceTo(other.X, other.Y); d <= maxDist && d < nearestDist {
			nearest, nearestDist = other, d
		}
		return true
	})
	return nearest, nearestDist
}

// npcTargetPosition resolves an NPC's current target, player or NPC, to a
// position. ok is false when the target is gone, dead or on another map.
func (e *Engine) npcTargetPosition(n *npc.NPC) (x, y int, ok bool) {
	if n.TargetID == 0 {
		return 0, 0, false
	}
	if val, isNPC := e.npcs.Load(n.TargetID); isNPC {
		t := val.(*npc.NPC)
		if t.IsAlive() && t.MapName == n.MapName {
			return t.X, t.Y, true
		}
		return 0, 0, false
	}
	if p := e.getPlayerByID(n.TargetID); p != nil && p.HP > 0 && p.MapName == n.MapName &&
		(p.Effects == nil || !p.Effects.HasEffect(magic.EffectInvisibility)) {
		return p.X, p.Y, true
	}
	return 0, 0, false
}

// npcAttackNPC resolves one NPC melee hit on another, using the same dice
// fields as NPC attacks on players.
func (e *Engine) npcAttackNPC(attacker, victim *npc.NPC, gm *mapdata.GameMap) {
	attacker.Direction = attacker.DirectionTo(victim.X, victim.Y)
	e.broadcastToNearby(gm, attacker.X, attacker.Y, -1, network.MsgMotionEvent, &pb.MotionEvent{
		ObjectId:  attacker.ObjectID,
		OwnerType: 2,
		Action:    3, // attack
		Direction: int32(attacker.Direction),
		Position:  &pb.Vec2{X: int32(attacker.X), Y: int32(attacker.Y)},
		Name:      attacker.Type.Name,
	})

	// Hit chance from DEX, as a ratio like the player formulas (kept between 10% and 95%)
	hitChance := 50 + (attacker.Type.DEX-victim.Type.DEX)/4
	hitChance = max(10, min(95, hitChance))
	miss := rand.Intn(100) >= hitChance

	dmg := 0
	if !miss {
		dmg = attacker.Type.AttackBonus
		for i := 0; i < attacker.Type.DiceThrow && attacker.Type.DiceRange > 0; i++ {
			dmg += 1 + rand.Intn(attacker.Type.DiceRange)
		}
		if dmg <= 0 && attacker.Type.MaxDamage > 0 { // types without dice data
			dmg = attacker.Type.MinDamage + rand.Intn(max(1, attacker.Type.MaxDamage-attacker.Type.MinDamage+1))
		}
		dmg -= victim.Type.Defense / 10
		dmg = max(dmg, 1)
	}

	killed := false
	if !miss {
		killed = victim.TakeDamage(dmg)
	}
	e.broadcastToNearby(gm, victim.X, victim.Y, -1, network.MsgDamageEvent, &pb.DamageEvent{
		AttackerId:  attacker.ObjectID,
		TargetId:    victim.ObjectID,
		TargetType:  2,
		Damage:      int32(dmg),
		Miss:        miss,
		TargetHp:    int32(victim.HP),
		TargetMaxHp: int32(victim.MaxHP),
	})

	// The victim fights back if it isn't busy with something else
	if !killed && victim.TargetID == 0 && npcHostileToNPC(victim, attacker) {
		victim.TargetID = attacker.ObjectID
		victim.State = npc.StateChase
	}
	if killed {
		e.handleNPCKilledByNPC(victim, attacker, gm)
	}
}

// handleNPCKilledByNPC is NPC death without a player killer: no XP, gold,
// loot or quest credit. Respawning (or removal for summoned NPCs) happens in
// the normal tick.
func (e *Engine) handleNPCKilledByNPC(victim, killer *npc.NPC, gm *mapdata.GameMap) {
	gm.ClearOwner(victim.X, victim.Y)
	e.broadcastToNearby(gm, victim.X, victim.Y, -1, network.MsgDeathEvent, &pb.DeathEvent{
		ObjectId:   victim.ObjectID,
		ObjectType: 2, // NPC
		KillerId:   killer.ObjectID,
		KillerName: killer.Type.Name,
		Position:   &pb.Vec2{X: int32(victim.X), Y: int32(victim.Y)},
	})
	e.broadcastToNearby(gm, victim.X, victim.Y, -1, network.MsgNpcDisappear, &pb.NpcDisappear{ObjectId: victim.ObjectID})
	if killer.TargetID == victim.ObjectID {
		e.npcDropTarget(killer, gm)
	}
}
