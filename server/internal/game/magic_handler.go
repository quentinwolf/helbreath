package game

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/juanrossi/hbonline/server/internal/items"
	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	"github.com/juanrossi/hbonline/server/internal/skills"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// handleSpellCast handles a spell cast request from a player.
//
// As in the original game, spells are cast at a tile. The client snaps a click
// to a nearby creature and sends its ID, otherwise just the tile.
func (e *Engine) handleSpellCast(client *network.Client, req *pb.SpellCastRequest) {
	p := e.getPlayerByClient(client)
	if p == nil || p.HP <= 0 {
		return
	}

	spellDef := magic.GetSpellDef(int(req.SpellId))
	if spellDef == nil {
		e.sendNotification(p, "Unknown spell", 2)
		return
	}

	canCast, reason := p.CanCastSpell(spellDef)
	if !canCast {
		e.sendNotification(p, reason, 2)
		return
	}

	gm, ok := e.maps[p.MapName]
	if !ok {
		return
	}

	tx, ty, targetID := e.resolveSpellTarget(p, req)

	// Check the target before spending anything, so a misclick costs no mana or cooldown
	if reason := e.validateSpellTarget(p, spellDef, tx, ty, targetID); reason != "" {
		e.sendNotification(p, reason, 2)
		return
	}

	// Deduct mana and start cooldown (god mode casts for free with no cooldown)
	if !p.GodMode {
		p.MP -= spellDef.ManaCost
		p.StartCooldown(spellDef.ID, spellDef.Cooldown)
	}

	offensive := spellDef.DealsDamage() || spellDef.IsField() || spellDef.Type == magic.SpellTypeDebuff
	if offensive && (tx != p.X || ty != p.Y) {
		p.Direction = directionTo(p.X, p.Y, tx, ty)
	}
	// Casting any spell breaks invisibility, as in the original (re-cast it to stay hidden)
	e.breakInvisibility(p)

	// Broadcast casting animation
	castMotion := &pb.MotionEvent{
		ObjectId:  p.ObjectID,
		OwnerType: 1,
		Action:    4, // magic
		Direction: int32(p.Direction),
		Position:  &pb.Vec2{X: int32(p.X), Y: int32(p.Y)},
		Name:      p.Name,
	}
	e.broadcastToNearby(gm, p.X, p.Y, -1, network.MsgMotionEvent, castMotion)

	switch {
	case e.castStatusSpell(p, spellDef, tx, ty, targetID, gm):
		// Recall, Summon Creature, Invisibility, Detect Invisibility, Polymorph
	case spellDef.IsField():
		e.castFieldSpell(p, spellDef, tx, ty, gm)
	case spellDef.DealsDamage():
		e.castDamageSpell(p, spellDef, tx, ty, gm)
	case spellDef.Type == magic.SpellTypeHeal:
		e.handleHealSpell(p, spellDef, tx, ty, targetID, gm)
	case spellDef.CfgType == magic.CfgTypeCreate:
		e.castCreateFood(p, spellDef, tx, ty, gm)
	case spellDef.Type == magic.SpellTypeBuff:
		e.castBuff(p, spellDef, targetID, gm)
	case e.castOriginalDebuff(p, spellDef, tx, ty, targetID, gm):
		// Stamina Drain, Hold Person, Paralyze, Possession, Confuse/Illusion family
	case spellDef.Type == magic.SpellTypeDebuff:
		e.handleDebuffSpell(p, spellDef, targetID, gm)
	}

	// Gain magic skill mastery
	if p.Skills.CanGainMastery(skills.SkillMagic) {
		// 20% chance per cast to gain mastery
		if rand.Intn(5) == 0 {
			p.Skills.GainMastery(skills.SkillMagic)
		}
	}

	e.sendStatUpdate(p)
}

// resolveSpellTarget returns the tile a spell lands on and whoever stands
// there (0 if nobody). A named target wins over the clicked tile, so a click
// near a moving monster still lands on it.
func (e *Engine) resolveSpellTarget(p *player.Player, req *pb.SpellCastRequest) (tx, ty int, targetID int32) {
	if req.TargetId != 0 {
		if x, y, ok := e.livingObjectPosition(p.MapName, req.TargetId); ok && req.TargetId != p.ObjectID {
			return x, y, req.TargetId
		}
	}
	if req.TargetPosition == nil {
		return p.X, p.Y, 0
	}
	tx, ty = int(req.TargetPosition.GetX()), int(req.TargetPosition.GetY())
	return tx, ty, e.livingObjectAt(p.MapName, tx, ty, p.ObjectID)
}

// livingObjectPosition finds a living NPC or player on the given map.
func (e *Engine) livingObjectPosition(mapName string, id int32) (x, y int, ok bool) {
	if val, isNPC := e.npcs.Load(id); isNPC {
		n := val.(*npc.NPC)
		if n.IsAlive() && n.MapName == mapName {
			return n.X, n.Y, true
		}
		return 0, 0, false
	}
	if t := e.getPlayerByID(id); t != nil && t.HP > 0 && t.MapName == mapName {
		return t.X, t.Y, true
	}
	return 0, 0, false
}

// livingObjectAt returns the living NPC or player standing on a tile, ignoring
// the given object (the caster).
func (e *Engine) livingObjectAt(mapName string, x, y int, ignore int32) int32 {
	var found int32
	e.npcs.Range(func(_, value any) bool {
		n := value.(*npc.NPC)
		if n.IsAlive() && n.MapName == mapName && n.X == x && n.Y == y {
			found = n.ObjectID
			return false
		}
		return true
	})
	if found != 0 {
		return found
	}
	e.players.Range(func(_, value any) bool {
		t := value.(*player.Player)
		if t.ObjectID != ignore && t.HP > 0 && t.MapName == mapName && t.X == x && t.Y == y {
			found = t.ObjectID
			return false
		}
		return true
	})
	return found
}

// validateSpellTarget returns "" when the cast can go ahead, or the message to
// show the player. It runs before mana or cooldown is spent.
func (e *Engine) validateSpellTarget(p *player.Player, spell *magic.SpellDef, tx, ty int, targetID int32) string {
	// (Summon Creature's table range is 1; it reaches 8 tiles, checked below.)
	if spell.Range > 0 && spell.CfgType != cfgTypeSummon && max(abs(tx-p.X), abs(ty-p.Y)) > spell.Range {
		return "Target out of range"
	}
	if reason := statusSpellCheck(p, spell, tx, ty); reason != "" {
		return reason
	}
	// Original PlayerMagicHandler: no attack magic from a no-attack area
	offensive := spell.DealsDamage() || spell.IsField() || spell.Type == magic.SpellTypeDebuff
	if offensive && mapdata.IsNoAttackTile(p.MapName, p.X, p.Y) {
		return "You can't cast attack spells in a safe area"
	}
	if spell.IsField() && fieldFreeMaps[p.MapName] {
		return "You cannot cast that in town"
	}
	if spell.CfgType == magic.CfgTypeCreate {
		if gm, ok := e.maps[p.MapName]; ok {
			return createFoodCheck(p, gm, tx, ty)
		}
	}
	if spell.Type == magic.SpellTypeHeal {
		return e.healSpellCheck(p, spell, tx, ty, targetID)
	}
	if !spell.NeedsTarget() {
		return ""
	}
	if targetID == 0 {
		return "No target selected"
	}
	if val, isNPC := e.npcs.Load(targetID); isNPC && !e.playerCanHurtNPC(p, val.(*npc.NPC)) {
		return "The guards are not your enemy"
	}
	if t := e.getPlayerByID(targetID); t != nil && !spellCanHitPlayer(p, t) {
		if sameSide(p, t) && t != p {
			return sameSideMessage(p)
		}
		return "No target selected"
	}
	return ""
}

// spellCanHitPlayer is the PvP rule for spells, the same one melee uses: never
// yourself, never your own side. Travellers count as a side of their own, so
// they can't hurt each other either (Mass spells used to kill nearby travellers).
func spellCanHitPlayer(caster, target *player.Player) bool {
	if target == caster || target.HP <= 0 || target.MapName != caster.MapName || target.GodMode {
		return false
	}
	return !sameSide(caster, target)
}

// sameSide: both Aresden, both Elvine, or both travellers.
func sameSide(a, b *player.Player) bool { return a.Side == b.Side }

// sameSideMessage is why you can't attack someone on your own side.
func sameSideMessage(p *player.Player) string {
	if p.Side == 0 {
		return "Travellers cannot attack each other"
	}
	return "Cannot attack a member of your own faction"
}

// spellTileHit records which of a spell's rolls land on one tile.
type spellTileHit struct{ centre, area, line bool }

// spellHitTiles lays out a damage spell's footprint the way the original
// server did: the clicked tile, a box around it, and for line spells the
// tiles 2-9 steps out from the caster towards the target plus their four
// neighbours.
func spellHitTiles(spell *magic.SpellDef, cx, cy, tx, ty int) map[[2]int]spellTileHit {
	hits := map[[2]int]spellTileHit{}
	mark := func(x, y int, f func(h *spellTileHit)) {
		h := hits[[2]int{x, y}]
		f(&h)
		hits[[2]int{x, y}] = h
	}

	if !spell.CentreDice.IsZero() {
		mark(tx, ty, func(h *spellTileHit) { h.centre = true })
	}
	if !spell.AreaDice.IsZero() {
		for y := ty - spell.AreaY; y <= ty+spell.AreaY; y++ {
			for x := tx - spell.AreaX; x <= tx+spell.AreaX; x++ {
				mark(x, y, func(h *spellTileHit) { h.area = true })
			}
		}
	}
	if spell.Line && !spell.LineDice.IsZero() {
		dist := max(abs(tx-cx), abs(ty-cy))
		for i := 2; i < 10 && dist > 0; i++ {
			lx := cx + int(math.Round(float64((tx-cx)*i)/float64(dist)))
			ly := cy + int(math.Round(float64((ty-cy)*i)/float64(dist)))
			for _, d := range [][2]int{{0, 0}, {-1, 0}, {1, 0}, {0, -1}, {0, 1}} {
				mark(lx+d[0], ly+d[1], func(h *spellTileHit) { h.line = true })
			}
			if abs(lx-tx) <= 1 && abs(ly-ty) <= 1 {
				break
			}
		}
	}
	return hits
}

// rollDice rolls an original-style damage roll: Count d Sides + Bonus.
func rollDice(d magic.Dice) int {
	total := d.Bonus
	for i := 0; i < d.Count && d.Sides > 0; i++ {
		total += 1 + rand.Intn(d.Sides)
	}
	return total
}

// rollSpellDamage is one victim's damage from the rolls that reach its tile.
func rollSpellDamage(p *player.Player, spell *magic.SpellDef, h spellTileHit) int {
	dmg := 0
	if h.centre {
		dmg += rollDice(spell.CentreDice)
	}
	if h.area {
		dmg += rollDice(spell.AreaDice)
	} else if h.line {
		dmg += rollDice(spell.LineDice)
	}
	if dmg <= 0 {
		return 0
	}
	if h.centre {
		dmg += p.EffectiveMAG()/3 + p.EffectiveINT()/4
	} else {
		dmg += p.EffectiveMAG()/4 + p.EffectiveINT()/5
	}
	dmg = applyElementBonus(dmg, spell.Element)
	if p.GodMode {
		dmg *= 1000
	}
	return dmg
}

// castDamageSpell lands a damage spell on a tile and hits every monster and
// enemy player inside its footprint.
func (e *Engine) castDamageSpell(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	hits := spellHitTiles(spell, p.X, p.Y, tx, ty)

	// One effect for the whole cast; each victim's number comes with its DamageEvent
	e.broadcastSpellEffect(p, spell, 0, p.X, p.Y, tx, ty, 0, 0, false, gm)

	var npcVictims []*npc.NPC
	e.npcs.Range(func(_, value any) bool {
		n := value.(*npc.NPC)
		if n.IsAlive() && n.MapName == p.MapName && e.playerCanHurtNPC(p, n) {
			if _, hit := hits[[2]int{n.X, n.Y}]; hit {
				npcVictims = append(npcVictims, n)
			}
		}
		return true
	})
	var playerVictims []*player.Player
	e.players.Range(func(_, value any) bool {
		t := value.(*player.Player)
		if spellCanHitPlayer(p, t) {
			if _, hit := hits[[2]int{t.X, t.Y}]; hit {
				playerVictims = append(playerVictims, t)
			}
		}
		return true
	})

	for _, n := range npcVictims {
		dmg := rollSpellDamage(p, spell, hits[[2]int{n.X, n.Y}])
		// Defense reduction (capped at 60% for magic)
		dmg -= min(n.Type.Defense/2, dmg*60/100)
		if dmg < 1 {
			dmg = 1
		}
		e.spellHurtNPC(p, n, dmg, gm)
	}

	for _, t := range playerVictims {
		dmg := rollSpellDamage(p, spell, hits[[2]int{t.X, t.Y}])
		// Protection From Magic halves spell damage; Absolute Magic Protection blocks it
		if t.Effects != nil && t.Effects.HasEffect(magic.EffectMagicProtection) {
			if t.Effects.GetEffectLevel(magic.EffectMagicProtection) >= 3 {
				dmg = 0
			} else {
				dmg /= 2
			}
		}
		if dmg <= 0 {
			continue
		}
		var freeze time.Duration
		if spell.ApplyEffect == magic.EffectIce {
			freeze = time.Duration(spell.Duration) * time.Second
		}
		e.spellHurtPlayer(p, t, dmg, freeze, max(spell.EffectLevel, 1), gm)
	}

	log.Printf("Spell %s at (%d,%d): hit %d monsters, %d players", spell.Name, tx, ty, len(npcVictims), len(playerVictims))
}

// spellHurtNPC deals a caster's spell damage to a monster: damage number, it
// turns on the caster, and loot/exp if it dies.
func (e *Engine) spellHurtNPC(p *player.Player, n *npc.NPC, dmg int, gm *mapdata.GameMap) {
	killed := n.TakeDamage(dmg)
	n.HeldUntil = time.Time{} // spell damage breaks Hold Person / Paralyze
	e.broadcastToNearby(gm, n.X, n.Y, -1, network.MsgDamageEvent, &pb.DamageEvent{
		AttackerId:  p.ObjectID,
		TargetId:    n.ObjectID,
		TargetType:  2,
		Damage:      int32(dmg),
		TargetHp:    int32(n.HP),
		TargetMaxHp: int32(n.MaxHP),
	})
	if n.IsAlive() && n.TargetID == 0 {
		n.TargetID = p.ObjectID
		n.State = npc.StateChase
	}
	if killed {
		e.handleNPCDeath(n, p, gm)
	}
}

// spellHurtPlayer deals a caster's spell damage to a player. freeze > 0 also
// freezes them (Ice spells) for that long at the given level.
func (e *Engine) spellHurtPlayer(p, t *player.Player, dmg int, freeze time.Duration, freezeLevel int, gm *mapdata.GameMap) {
	if mapdata.IsNoAttackTile(t.MapName, t.X, t.Y) {
		return // original Effect_Damage_Spot: players in a no-attack area aren't hurt
	}
	t.HP -= dmg
	killed := t.HP <= 0
	if killed {
		t.HP = 0
	}
	e.broadcastToNearby(gm, t.X, t.Y, -1, network.MsgDamageEvent, &pb.DamageEvent{
		AttackerId:  p.ObjectID,
		TargetId:    t.ObjectID,
		TargetType:  1,
		Damage:      int32(dmg),
		TargetHp:    int32(t.HP),
		TargetMaxHp: int32(t.MaxHP),
	})
	e.cancelLogoutOnDamage(t)
	// Spell damage breaks Hold Person / Paralyze (melee doesn't, as in the original)
	if t.Effects != nil && t.Effects.HasEffect(magic.EffectHold) {
		t.Effects.RemoveEffect(magic.EffectHold)
		e.sendStatusOff(t, magic.EffectHold)
	}
	if !killed && freeze > 0 && t.Effects != nil {
		t.Effects.AddEffect(&magic.ActiveEffect{
			Type:      magic.EffectIce,
			Level:     freezeLevel,
			ExpiresAt: time.Now().Add(freeze),
			SourceID:  p.ObjectID,
		})
	}
	e.sendStatUpdate(t)
	if killed {
		e.onPlayerKilledByPlayer(p, t)
	}
}

// handleBuffSpell casts a buff on yourself.
func (e *Engine) handleBuffSpell(p *player.Player, spell *magic.SpellDef, gm *mapdata.GameMap) {
	e.buffPlayer(p, p, spell, gm)
}

// buffPlayer puts caster's buff on p (the caster or whoever they clicked).
func (e *Engine) buffPlayer(caster, p *player.Player, spell *magic.SpellDef, gm *mapdata.GameMap) {
	// Apply status effect if the spell has one
	if spell.ApplyEffect != 0 && p.Effects != nil {
		eff := &magic.ActiveEffect{
			Type:      spell.ApplyEffect,
			Level:     spell.EffectLevel,
			ExpiresAt: time.Now().Add(time.Duration(spell.Duration) * time.Second),
			SourceID:  caster.ObjectID,
		}
		if spell.TickDamage > 0 {
			eff.TickDamage = spell.TickDamage
			eff.TickInterval = time.Duration(spell.TickIntervalMs) * time.Millisecond
			eff.LastTick = time.Now()
		}
		p.Effects.AddEffect(eff)
		if spell.ApplyEffect == magic.EffectBerserk {
			e.refreshPlayerAppearance(p) // everyone sees the glow
		}
		log.Printf("Player %s gained effect %s (level %d for %ds)", p.Name, spell.Name, spell.EffectLevel, spell.Duration)
	}

	// Apply stat buff if the spell has one
	if spell.BuffStat > 0 {
		buff := p.Buffs.AddBuff(spell.ID, spell.Name, spell.BuffStat, spell.BuffAmount, spell.Duration)

		// Send buff update to player
		buffUpdate := &pb.BuffUpdate{
			ObjectId:         p.ObjectID,
			SpellId:          int32(spell.ID),
			Name:             spell.Name,
			StatType:         int32(spell.BuffStat),
			Amount:           int32(spell.BuffAmount),
			RemainingSeconds: int32(buff.RemainingSeconds()),
		}
		data, _ := network.Encode(network.MsgBuffUpdate, buffUpdate)
		p.Send(data)

		log.Printf("Player %s gained buff %s (+%d to stat %d for %ds)", p.Name, spell.Name, spell.BuffAmount, spell.BuffStat, spell.Duration)
	}

	if p != caster {
		e.sendNotification(p, caster.Name+" cast "+spell.Name+" on you", 0)
	} else if (spell.ApplyEffect != 0 || spell.BuffStat > 0) && spell.Duration > 0 {
		e.sendNotification(p, fmt.Sprintf("%s is on you for %d seconds", spell.Name, spell.Duration), 0)
	}
	e.broadcastSpellEffect(caster, spell, p.ObjectID, caster.X, caster.Y, p.X, p.Y, 0, 0, false, gm)
}

// handleDebuffSpell handles debuff spells on targets.
func (e *Engine) handleDebuffSpell(p *player.Player, spell *magic.SpellDef, targetID int32, gm *mapdata.GameMap) {
	if targetID == 0 {
		e.sendNotification(p, "No target selected", 2)
		return
	}

	// Check if target is NPC - debuffs don't apply stat modifiers to NPCs,
	// but we still show the effect and can slow them etc. in future
	npcVal, isNPC := e.npcs.Load(targetID)
	if isNPC {
		n := npcVal.(*npc.NPC)
		if !n.IsAlive() || n.MapName != p.MapName {
			return
		}
		dist := n.DistanceTo(p.X, p.Y)
		if dist > spell.Range {
			e.sendNotification(p, "Target out of range", 2)
			return
		}

		// Check magic resistance for NPC (use NPC INT as resistance)
		if spell.ApplyEffect != 0 && magic.CheckMagicResistance(p.EffectiveMAG(), n.Type.INT, spell.EffectLevel) {
			e.broadcastSpellEffect(p, spell, targetID, p.X, p.Y, n.X, n.Y, 0, 0, true, gm)
			e.sendNotification(p, n.Type.Name+" resisted the spell", 2)
			log.Printf("NPC %s resisted %s from player %s", n.Type.Name, spell.Name, p.Name)
			return
		}

		e.broadcastSpellEffect(p, spell, targetID, p.X, p.Y, n.X, n.Y, 0, 0, false, gm)
		log.Printf("Player %s cast %s on NPC %s", p.Name, spell.Name, n.Type.Name)
		return
	}

	// PvP debuffs: apply status effect to target player
	targetPlayer := e.getPlayerByID(targetID)
	if targetPlayer != nil && targetPlayer.HP > 0 && targetPlayer.MapName == p.MapName {
		dist := abs(p.X-targetPlayer.X) + abs(p.Y-targetPlayer.Y)
		if dist > spell.Range {
			e.sendNotification(p, "Target out of range", 2)
			return
		}

		// Check magic resistance
		if spell.ApplyEffect != 0 && magic.CheckMagicResistance(p.EffectiveMAG(), targetPlayer.EffectiveINT(), spell.EffectLevel) {
			e.broadcastSpellEffect(p, spell, targetID, p.X, p.Y, targetPlayer.X, targetPlayer.Y, 0, 0, true, gm)
			e.sendNotification(p, targetPlayer.Name+" resisted the spell", 2)
			e.sendNotification(targetPlayer, "You resisted "+spell.Name, 3)
			log.Printf("Player %s resisted %s from player %s", targetPlayer.Name, spell.Name, p.Name)
			return
		}

		// Apply status effect
		if spell.ApplyEffect != 0 && targetPlayer.Effects != nil {
			eff := &magic.ActiveEffect{
				Type:      spell.ApplyEffect,
				Level:     spell.EffectLevel,
				ExpiresAt: time.Now().Add(time.Duration(spell.Duration) * time.Second),
				SourceID:  p.ObjectID,
			}
			if spell.TickDamage > 0 {
				eff.TickDamage = spell.TickDamage
				eff.TickInterval = time.Duration(spell.TickIntervalMs) * time.Millisecond
				eff.LastTick = time.Now()
			}
			targetPlayer.Effects.AddEffect(eff)
			e.sendNotification(targetPlayer, "You have been affected by "+spell.Name, 2)
		}

		// Apply stat debuff
		if spell.BuffStat > 0 {
			targetPlayer.Buffs.AddBuff(spell.ID, spell.Name, spell.BuffStat, spell.BuffAmount, spell.Duration)
		}

		// Cancel logout if the target was hit by a hostile spell
		e.cancelLogoutOnDamage(targetPlayer)

		e.broadcastSpellEffect(p, spell, targetID, p.X, p.Y, targetPlayer.X, targetPlayer.Y, 0, 0, false, gm)
		log.Printf("Player %s cast %s on player %s", p.Name, spell.Name, targetPlayer.Name)
	}
}

// sendSpellCatalog sends the full spell catalog with learning requirements to the player.
func (e *Engine) sendSpellCatalog(p *player.Player) {
	catalog := &pb.SpellCatalog{}
	for _, def := range magic.SpellDB {
		catalog.Spells = append(catalog.Spells, &pb.SpellCatalogEntry{
			SpellId:   int32(def.ID),
			Name:      def.Name,
			SpellType: int32(def.Type),
			ManaCost:  int32(def.ManaCost),
			ReqLevel:  int32(def.ReqLevel),
			ReqMag:    int32(def.ReqMAG),
			ReqInt:    int32(def.ReqINT),
			Learned:   p.LearnedSpells[def.ID],
		})
	}
	data, _ := network.Encode(network.MsgSpellCatalog, catalog)
	p.Send(data)
}

// handleLearnSpell handles learning a new spell.
func (e *Engine) handleLearnSpell(client *network.Client, req *pb.LearnSpellRequest) {
	p := e.getPlayerByClient(client)
	if p == nil {
		return
	}

	spellDef := magic.GetSpellDef(int(req.SpellId))
	if spellDef == nil {
		e.sendNotification(p, "Unknown spell", 2)
		return
	}

	if p.LearnedSpells[spellDef.ID] {
		e.sendNotification(p, "You already know this spell", 2)
		return
	}

	// Check requirements
	if p.Level < spellDef.ReqLevel {
		e.sendNotification(p, fmt.Sprintf("Requires level %d", spellDef.ReqLevel), 2)
		return
	}
	if p.MAG < spellDef.ReqMAG {
		e.sendNotification(p, fmt.Sprintf("Requires %d MAG", spellDef.ReqMAG), 2)
		return
	}
	if p.INT < spellDef.ReqINT {
		e.sendNotification(p, fmt.Sprintf("Requires %d INT", spellDef.ReqINT), 2)
		return
	}

	// Gandalf sells spells (the original's magic shop); GMs can learn anywhere.
	cost := int64(spellDef.GoldCost)
	isGM := p.AdminLevel >= 1
	if !isGM && !e.nearWizard(p) {
		e.sendNotification(p, "Visit Gandalf in the Wizard Tower to learn spells", 2)
		return
	}
	if cost < 0 {
		if !isGM {
			e.sendNotification(p, fmt.Sprintf("Gandalf doesn't teach %s: it's learnt from a manual", spellDef.Name), 2)
			return
		}
		cost = 0 // not sold: free for GMs
	}
	if p.Gold < cost {
		e.sendNotification(p, fmt.Sprintf("%s costs %d gold", spellDef.Name, cost), 2)
		return
	}

	p.Gold -= cost
	p.LearnSpell(spellDef.ID)
	if cost > 0 {
		e.sendStatUpdate(p) // gold first, so the Learn tab redraws with it
	}
	e.sendSpellList(p)
	e.sendSpellCatalog(p)
	if cost > 0 {
		e.sendNotification(p, fmt.Sprintf("Learned %s for %d gold!", spellDef.Name, cost), 3)
	} else {
		e.sendNotification(p, fmt.Sprintf("Learned %s!", spellDef.Name), 3)
	}
	log.Printf("Player %s learned spell %s (%d gold)", p.Name, spellDef.Name, cost)
}

// wizardReach is how close to Gandalf you must be to buy a spell: anywhere in
// his tower's main room, so the Learn panel keeps working while you browse.
const wizardReach = 10

// nearWizard reports whether a living Gandalf stands within wizardReach.
func (e *Engine) nearWizard(p *player.Player) bool {
	near := false
	e.npcs.Range(func(_, v any) bool {
		n := v.(*npc.NPC)
		if n.Type.ID == npcWizard && n.IsAlive() && n.MapName == p.MapName && n.DistanceTo(p.X, p.Y) <= wizardReach {
			near = true
			return false
		}
		return true
	})
	return near
}

// handleSkillUse handles using a gathering/crafting skill.
func (e *Engine) handleSkillUse(client *network.Client, req *pb.SkillUseRequest) {
	p := e.getPlayerByClient(client)
	if p == nil || p.HP <= 0 {
		return
	}

	skillID := skills.SkillID(req.SkillId)
	skillDef := skills.GetSkillDef(skillID)
	if skillDef == nil {
		e.sendNotification(p, "Unknown skill", 2)
		return
	}

	mastery := p.Skills.GetMastery(skillID)

	switch skillID {
	case skills.SkillMining:
		e.handleMining(p, mastery)
	case skills.SkillFishing:
		e.handleFishing(p, mastery)
	default:
		e.sendNotification(p, "This skill cannot be used directly", 2)
	}
}

// handleMining performs a mining action.
func (e *Engine) handleMining(p *player.Player, mastery int) {
	// Check for pickaxe equipped (future: specific tool items)
	// For now just do the skill check
	if p.SP < 5 {
		e.sendNotification(p, "Not enough stamina", 2)
		return
	}
	p.SP -= 5

	success := skills.SkillCheck(mastery+20, 10) // base +20 for easier starting
	result := &pb.SkillResultEvent{
		SkillId: int32(skills.SkillMining),
	}

	if success {
		// Give iron ore
		oreDef := items.GetItemDef(210) // Iron Ore
		if oreDef != nil {
			ore := items.NewItem(oreDef, 1)
			slot := p.Inventory.AddItem(ore)
			if slot >= 0 {
				result.Success = true
				result.Message = "You mined some iron ore!"
				result.ItemGainedId = 210
				e.sendInventoryUpdate(p)
			} else {
				result.Message = "Inventory full!"
			}
		}
	} else {
		result.Message = "You failed to mine anything useful."
	}

	// Try mastery gain
	newMastery, gained := p.Skills.GainMastery(skills.SkillMining)
	if gained {
		result.NewMastery = int32(newMastery)
	}

	data, _ := network.Encode(network.MsgSkillResult, result)
	p.Send(data)
	e.sendStatUpdate(p)
}

// handleFishing performs a fishing action.
func (e *Engine) handleFishing(p *player.Player, mastery int) {
	if p.SP < 3 {
		e.sendNotification(p, "Not enough stamina", 2)
		return
	}
	p.SP -= 3

	success := skills.SkillCheck(mastery+25, 5) // slightly easier than mining
	result := &pb.SkillResultEvent{
		SkillId: int32(skills.SkillFishing),
	}

	if success {
		fishDef := items.GetItemDef(220) // Raw Fish
		if fishDef != nil {
			fish := items.NewItem(fishDef, 1)
			slot := p.Inventory.AddItem(fish)
			if slot >= 0 {
				result.Success = true
				result.Message = "You caught a fish!"
				result.ItemGainedId = 220
				e.sendInventoryUpdate(p)
			} else {
				result.Message = "Inventory full!"
			}
		}
	} else {
		result.Message = "The fish got away..."
	}

	newMastery, gained := p.Skills.GainMastery(skills.SkillFishing)
	if gained {
		result.NewMastery = int32(newMastery)
	}

	data, _ := network.Encode(network.MsgSkillResult, result)
	p.Send(data)
	e.sendStatUpdate(p)
}

// handleCraft handles crafting an item from a recipe.
func (e *Engine) handleCraft(client *network.Client, req *pb.CraftRequest) {
	p := e.getPlayerByClient(client)
	if p == nil {
		return
	}

	recipe := skills.GetRecipe(int(req.RecipeId))
	if recipe == nil {
		e.sendNotification(p, "Unknown recipe", 2)
		return
	}

	// Check skill mastery requirement
	mastery := p.Skills.GetMastery(recipe.Skill)
	if mastery < recipe.ReqMastery {
		e.sendNotification(p, fmt.Sprintf("Requires %d %s mastery", recipe.ReqMastery,
			skills.GetSkillDef(recipe.Skill).Name), 2)
		return
	}

	// Check all inputs are available
	for _, input := range recipe.Inputs {
		if p.Inventory.CountItem(input.ItemID) < input.Count {
			def := items.GetItemDef(input.ItemID)
			name := "item"
			if def != nil {
				name = def.Name
			}
			e.sendNotification(p, fmt.Sprintf("Need %d %s", input.Count, name), 2)
			return
		}
	}

	// Check inventory space for output
	if !p.Inventory.HasSpace() {
		e.sendNotification(p, "Inventory full", 2)
		return
	}

	// Consume inputs
	for _, input := range recipe.Inputs {
		remaining := input.Count
		for i := 0; i < items.MaxInventorySlots && remaining > 0; i++ {
			item := p.Inventory.GetItem(i)
			if item == nil || item.DefID != input.ItemID {
				continue
			}
			remove := remaining
			if remove > item.Count {
				remove = item.Count
			}
			p.Inventory.RemoveItem(i, remove)
			remaining -= remove
		}
	}

	// Skill check for success
	result := &pb.CraftResult{}
	if skills.SkillCheck(mastery+30, recipe.ReqMastery/2) {
		outputDef := items.GetItemDef(recipe.OutputID)
		if outputDef != nil {
			newItem := items.NewItem(outputDef, recipe.OutputCount)
			p.Inventory.AddItem(newItem)
			result.Success = true
			result.Message = fmt.Sprintf("Created %s!", outputDef.Name)
			result.ItemId = int32(recipe.OutputID)
			result.Count = int32(recipe.OutputCount)
		}
	} else {
		result.Message = "Crafting failed! Materials were consumed."
	}

	// Try mastery gain
	p.Skills.GainMastery(recipe.Skill)

	data, _ := network.Encode(network.MsgCraftResult, result)
	p.Send(data)
	e.sendInventoryUpdate(p)
	e.sendSkillList(p)
}

// broadcastSpellEffect sends a spell visual effect to nearby players.
func (e *Engine) broadcastSpellEffect(caster *player.Player, spell *magic.SpellDef,
	targetID int32, cx, cy, tx, ty int, damage, heal int, miss bool, gm *mapdata.GameMap) {

	effect := &pb.SpellEffectEvent{
		CasterId:       caster.ObjectID,
		SpellId:        int32(spell.ID),
		TargetId:       targetID,
		CasterPosition: &pb.Vec2{X: int32(cx), Y: int32(cy)},
		TargetPosition: &pb.Vec2{X: int32(tx), Y: int32(ty)},
		Damage:         int32(damage),
		HealAmount:     int32(heal),
		Miss:           miss,
		SpriteId:       int32(spell.SpriteID),
		SoundId:        int32(spell.SoundID),
	}
	e.broadcastToNearby(gm, cx, cy, -1, network.MsgSpellEffect, effect)
}

// sendSpellList sends the player's known spells.
func (e *Engine) sendSpellList(p *player.Player) {
	update := p.ToSpellList()
	data, _ := network.Encode(network.MsgSpellList, update)
	p.Send(data)
}

// sendSkillList sends the player's skill masteries.
func (e *Engine) sendSkillList(p *player.Player) {
	update := p.ToSkillList()
	data, _ := network.Encode(network.MsgSkillList, update)
	p.Send(data)
}

// processEffectTicks processes status effect ticks for all players.
func (e *Engine) processEffectTicks(now time.Time) {
	e.players.Range(func(key, value any) bool {
		p := value.(*player.Player)
		if p.Effects == nil || p.HP <= 0 {
			return true
		}

		events := p.Effects.ProcessTick(now)
		for _, evt := range events {
			switch evt.EventType {
			case magic.EffectEventPoisonDamage:
				if p.GodMode {
					continue // invulnerable: poison does nothing
				}
				p.HP -= evt.Damage
				if p.HP <= 0 {
					p.HP = 0
				}

				// Send damage event for poison tick
				gm, ok := e.maps[p.MapName]
				if ok {
					dmgEvent := &pb.DamageEvent{
						AttackerId:  0, // environmental/poison
						TargetId:    p.ObjectID,
						TargetType:  1, // player
						Damage:      int32(evt.Damage),
						TargetHp:    int32(p.HP),
						TargetMaxHp: int32(p.MaxHP),
					}
					e.broadcastToNearby(gm, p.X, p.Y, -1, network.MsgDamageEvent, dmgEvent)
				}
				e.sendStatUpdate(p)

				// Handle death by poison
				if p.HP <= 0 {
					e.handlePlayerDeath(p, 0, "Poison", true)
				}

			case magic.EffectEventExpired:
				// Notify player about effect expiring
				effectName := effectTypeName(evt.EffectType)
				e.sendNotification(p, effectName+" has worn off", 0)
				if evt.EffectType == magic.EffectInvisibility || evt.EffectType == magic.EffectPolymorph || evt.EffectType == magic.EffectBerserk {
					e.refreshPlayerAppearance(p)
				}
				e.sendStatusOff(p, evt.EffectType)
			}
		}
		return true
	})
}

// effectTypeName returns a human-readable name for an effect type.
func effectTypeName(t magic.EffectType) string {
	switch t {
	case magic.EffectPoison:
		return "Poison"
	case magic.EffectIce:
		return "Freeze"
	case magic.EffectBerserk:
		return "Berserk"
	case magic.EffectInvisibility:
		return "Invisibility"
	case magic.EffectPolymorph:
		return "Polymorph"
	case magic.EffectInhibition:
		return "Silence"
	case magic.EffectDefenseShield:
		return "Defense Shield"
	case magic.EffectMagicProtection:
		return "Magic Protection"
	case magic.EffectHold:
		return "Hold"
	case magic.EffectConfusion:
		return "Confusion"
	default:
		return "Status Effect"
	}
}

// checkBuffExpiry checks all players for expired buffs and removes them.
func (e *Engine) checkBuffExpiry() {
	e.players.Range(func(key, value any) bool {
		p := value.(*player.Player)
		expired := p.Buffs.CleanExpired()
		for _, buff := range expired {
			// Send buff removal notification
			buffUpdate := &pb.BuffUpdate{
				ObjectId: p.ObjectID,
				SpellId:  int32(buff.SpellID),
				Name:     buff.Name,
				StatType: int32(buff.StatType),
				Amount:   int32(buff.Amount),
				Removed:  true,
			}
			data, _ := network.Encode(network.MsgBuffUpdate, buffUpdate)
			p.Send(data)

			e.sendNotification(p, buff.Name+" has worn off", 0)
		}
		return true
	})
}

// applyElementBonus adds a small element-based bonus to damage.
func applyElementBonus(damage int, element magic.SpellElement) int {
	// 10% bonus for elemental spells
	if element != magic.ElementNone {
		damage = damage * 110 / 100
	}
	return damage
}
