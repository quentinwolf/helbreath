package game

import (
	"math/rand"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// NPC-vs-NPC combat: town guards fight monsters, and monsters fight back.
// Shopkeepers and other non-combat NPCs stay out of it, and monsters don't
// fight each other.

// isGuardType reports whether an NPC type is a fighting non-monster (a guard).
func isGuardType(t *npc.NpcType) bool {
	return t.Side != npc.SideMonster && t.AggroRange > 0 && !npc.IsShopNPC(t.ID)
}

// npcHostileToNPC reports whether a would attack b.
func npcHostileToNPC(a, b *npc.NPC) bool {
	if a == b {
		return false
	}
	aMonster, bMonster := a.Type.Side == npc.SideMonster, b.Type.Side == npc.SideMonster
	return (aMonster && isGuardType(b.Type)) || (isGuardType(a.Type) && bMonster)
}

// findNearestEnemyNPC returns the closest living NPC that n would attack.
func (e *Engine) findNearestEnemyNPC(n *npc.NPC, maxDist int) (*npc.NPC, int) {
	if n.Type.Side != npc.SideMonster && !isGuardType(n.Type) {
		return nil, 999
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
	if p := e.getPlayerByID(n.TargetID); p != nil && p.HP > 0 && p.MapName == n.MapName {
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
		killer.TargetID = 0
		killer.State = npc.StateIdle
	}
}
