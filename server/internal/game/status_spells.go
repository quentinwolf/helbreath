package game

import (
	"log"
	"math/rand"
	"time"

	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	"github.com/juanrossi/hbonline/server/internal/skills"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// Spells that change who you are or where you stand, ported from the original
// server's PlayerMagicHandler: Recall (DEF_MAGICTYPE_TELEPORT), Summon Creature
// (SUMMON), Invisibility and Detect Invisibility (INVISIBILITY, sValue4 1 and 2)
// and Polymorph (POLYMORPH). Keyed by the spell's Magic.cfg type.
const (
	cfgTypeTeleport     = 8
	cfgTypeSummon       = 9
	cfgTypeInvisibility = 13
	cfgTypePolymorph    = 20
)

// detectInvisibilityRadius: Detect Invisibility reveals everyone within 8 tiles of the target.
const detectInvisibilityRadius = 8

// summonLifetime: summoned creatures vanish after this (the original's DEF_SUMMONTIME,
// 5 minutes in the standard source; its header isn't in our reference copy).
const summonLifetime = 5 * time.Minute

// summonRange: Summon Creature calls its creature onto a tile up to 8 tiles away.
const summonRange = 8

// summonCreature is one creature Summon Creature can call, and the caster level it
// suits (roughly the level of players who hunt it).
type summonCreature struct {
	Name  string
	Level int
}

// summonCreatures, weakest first. The original rolled off Magery and topped out at a
// Cyclops; here the roll follows the caster's level so strong players get creatures
// that keep up with them.
var summonCreatures = []summonCreature{
	{"Slime", 1}, {"Giant-Ant", 5}, {"Amphis", 10}, {"Orc", 15}, {"Zombie", 20},
	{"Skeleton", 30}, {"Clay-Golem", 40}, {"Stone-Golem", 45}, {"Orc-Mage", 50},
	{"Hellbound", 55}, {"Troll", 60}, {"Cyclops", 65}, {"Ogre", 75}, {"Liche", 90},
	{"Stalker", 100}, {"WereWolf", 110}, {"Ettin", 140}, {"MasterMage-Orc", 150},
	{"Demon", 165}, {"Gagoyle", 180},
}

// recallTowns: where Recall takes a citizen (the town's first initial-point).
var recallTowns = map[int]mapdata.TeleportDest{
	1: {DestMap: "aresden", DestX: 170, DestY: 146},
	2: {DestMap: "elvine", DestX: 170, DestY: 146},
}

func isDetectInvisibility(spell *magic.SpellDef) bool {
	return spell.CfgType == cfgTypeInvisibility && spell.ApplyEffect != magic.EffectInvisibility
}

// statusSpellCheck refuses these spells before any mana is spent. Travellers have no
// town to Recall to. (The original also kept travellers from both invisibility
// spells; we let everyone use them.)
func statusSpellCheck(p *player.Player, spell *magic.SpellDef, tx, ty int) string {
	switch spell.CfgType {
	case cfgTypeTeleport:
		if p.Side == 0 {
			return "Only citizens can Recall"
		}
	case cfgTypeSummon:
		if max(abs(tx-p.X), abs(ty-p.Y)) > summonRange {
			return "Target out of range"
		}
	case cfgTypeInvisibility, cfgTypePolymorph:
		if max(abs(tx-p.X), abs(ty-p.Y)) > magic.MaxCastRange {
			return "Target out of range"
		}
	}
	return ""
}

// castStatusSpell runs one of these spells and reports whether it was one.
func (e *Engine) castStatusSpell(p *player.Player, spell *magic.SpellDef, tx, ty int, targetID int32, gm *mapdata.GameMap) bool {
	switch {
	case spell.CfgType == cfgTypeTeleport:
		e.castRecall(p, spell, gm)
	case spell.CfgType == cfgTypeSummon:
		e.castSummon(p, spell, tx, ty, gm)
	case isDetectInvisibility(spell):
		e.castDetectInvisibility(p, spell, tx, ty, gm)
	case spell.CfgType == cfgTypeInvisibility:
		e.castInvisibility(p, spell, targetID, gm)
	case spell.CfgType == cfgTypePolymorph:
		e.castPolymorph(p, spell, targetID, gm)
	default:
		return false
	}
	return true
}

// refreshPlayerAppearance re-sends a player's PlayerAppear to them and to everyone
// nearby, so a status change (invisibility, polymorph) shows at once.
func (e *Engine) refreshPlayerAppearance(p *player.Player) {
	appear := p.ToPlayerAppear()
	if data, err := network.Encode(network.MsgPlayerAppear, appear); err == nil {
		p.Send(data)
	}
	if gm, ok := e.maps[p.MapName]; ok {
		e.broadcastToNearby(gm, p.X, p.Y, p.ObjectID, network.MsgPlayerAppear, appear)
	}
}

// breakInvisibility ends a player's invisibility (they attacked or cast a spell).
func (e *Engine) breakInvisibility(p *player.Player) {
	if p == nil || p.Effects == nil || !p.Effects.HasEffect(magic.EffectInvisibility) {
		return
	}
	p.Effects.RemoveEffect(magic.EffectInvisibility)
	e.refreshPlayerAppearance(p)
}

// spellTargetPlayer is the player a self-or-other spell lands on: whoever stands on
// the clicked tile, otherwise the caster.
func (e *Engine) spellTargetPlayer(p *player.Player, targetID int32) *player.Player {
	if targetID != 0 && targetID != p.ObjectID {
		if t := e.getPlayerByID(targetID); t != nil && t.HP > 0 && t.MapName == p.MapName {
			return t
		}
		return nil
	}
	return p
}

// castRecall sends a citizen to their home town.
func (e *Engine) castRecall(p *player.Player, spell *magic.SpellDef, gm *mapdata.GameMap) {
	dest, ok := recallTowns[p.Side]
	if !ok {
		return
	}
	e.broadcastSpellEffect(p, spell, p.ObjectID, p.X, p.Y, p.X, p.Y, 0, 0, false, gm)
	e.handleTeleport(p, gm, dest)
}

// castInvisibility hides the player on the target tile (or the caster).
func (e *Engine) castInvisibility(p *player.Player, spell *magic.SpellDef, targetID int32, gm *mapdata.GameMap) {
	t := e.spellTargetPlayer(p, targetID)
	if t == nil || t.Effects == nil || t.Effects.HasEffect(magic.EffectInvisibility) {
		return
	}
	t.Effects.AddEffect(&magic.ActiveEffect{
		Type:      magic.EffectInvisibility,
		Level:     1,
		ExpiresAt: time.Now().Add(time.Duration(spell.Duration) * time.Second),
		SourceID:  p.ObjectID,
	})
	e.broadcastSpellEffect(p, spell, t.ObjectID, p.X, p.Y, t.X, t.Y, 0, 0, false, gm)
	e.refreshPlayerAppearance(t)
	if t != p {
		e.sendNotification(t, p.Name+" made you invisible", 0)
	}
}

// castDetectInvisibility reveals every invisible player within 8 tiles of the target.
func (e *Engine) castDetectInvisibility(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	e.broadcastSpellEffect(p, spell, 0, p.X, p.Y, tx, ty, 0, 0, false, gm)
	revealed := 0
	e.players.Range(func(_, value any) bool {
		t := value.(*player.Player)
		if t.MapName != p.MapName || t.HP <= 0 || t.Effects == nil || !t.Effects.HasEffect(magic.EffectInvisibility) {
			return true
		}
		if max(abs(t.X-tx), abs(t.Y-ty)) > detectInvisibilityRadius {
			return true
		}
		e.breakInvisibility(t)
		if t != p {
			revealed++
			e.sendNotification(t, "Your invisibility was dispelled", 2)
		}
		return true
	})
	if revealed > 0 {
		log.Printf("Player %s's Detect Invisibility revealed %d players", p.Name, revealed)
	}
}

// castPolymorph turns the target (player or monster, or the caster) into a Zombie
// for the spell's duration. It's only a disguise; stats are unchanged.
func (e *Engine) castPolymorph(p *player.Player, spell *magic.SpellDef, targetID int32, gm *mapdata.GameMap) {
	until := time.Now().Add(time.Duration(spell.Duration) * time.Second)
	if val, isNPC := e.npcs.Load(targetID); isNPC {
		n := val.(*npc.NPC)
		if !n.IsAlive() || n.MapName != p.MapName || npc.IsShopNPC(n.Type.ID) || n.IsPolymorphed(time.Now()) {
			return
		}
		n.PolymorphUntil = until
		e.broadcastSpellEffect(p, spell, n.ObjectID, p.X, p.Y, n.X, n.Y, 0, 0, false, gm)
		e.broadcastToNearby(gm, n.X, n.Y, -1, network.MsgNpcAppear, n.ToNpcAppear())
		return
	}
	t := e.spellTargetPlayer(p, targetID)
	if t == nil || t.Effects == nil || t.Effects.HasEffect(magic.EffectPolymorph) {
		return
	}
	t.Effects.AddEffect(&magic.ActiveEffect{
		Type:      magic.EffectPolymorph,
		Level:     1,
		ExpiresAt: until,
		SourceID:  p.ObjectID,
	})
	e.broadcastSpellEffect(p, spell, t.ObjectID, p.X, p.Y, t.X, t.Y, 0, 0, false, gm)
	e.refreshPlayerAppearance(t)
}

// summonsOf counts the living creatures a player has summoned.
func (e *Engine) summonsOf(masterID int32) int {
	count := 0
	e.npcs.Range(func(_, value any) bool {
		if n := value.(*npc.NPC); n.MasterID == masterID && n.IsAlive() {
			count++
		}
		return true
	})
	return count
}

// summonRoll picks a creature for a caster of this level: a d6 over the strongest
// three they qualify for (1-3 the strongest, 4-5 the next, 6 the third), so most
// casts bring a creature at their level and some a step or two below.
func summonRoll(level int) string {
	top := 0
	for i, c := range summonCreatures {
		if c.Level <= level {
			top = i
		}
	}
	step := 0
	switch roll := 1 + rand.Intn(6); {
	case roll >= 6:
		step = 2
	case roll >= 4:
		step = 1
	}
	return summonCreatures[max(0, top-step)].Name
}

// castSummon calls a creature onto the target tile (up to 8 tiles away) that then
// follows the caster and fights monsters for them. You can keep Magery/20 of them
// (at least one) at a time.
func (e *Engine) castSummon(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	magery := p.Skills.GetMastery(skills.SkillMagic)
	if e.summonsOf(p.ObjectID) >= max(1, magery/20) {
		e.sendNotification(p, "You cannot control any more creatures", 2)
		return
	}
	name := summonRoll(p.Level)
	nt := npc.NpcTypeByName(name)
	if nt == nil {
		log.Printf("Summon Creature: no NPC type %q", name)
		return
	}

	// On the clicked tile if it's free, else the nearest free one; never on the caster
	if (tx == p.X && ty == p.Y) || max(abs(tx-p.X), abs(ty-p.Y)) > summonRange {
		tx, ty = p.X+1, p.Y
	}
	x, y := tx, ty
	if !gm.IsWalkable(x, y) {
		x, y = e.findWalkable(gm, tx, ty)
	}
	n := npc.NewNPC(e.nextObjID.Add(1), nt, p.MapName, x, y)
	n.Summoned = true
	n.MasterID = p.ObjectID
	n.MasterSide = p.Side
	n.SummonExpires = time.Now().Add(summonLifetime)
	e.npcs.Store(n.ObjectID, n)
	gm.SetOwner(x, y, n.ObjectID)

	e.broadcastSpellEffect(p, spell, n.ObjectID, p.X, p.Y, x, y, 0, 0, false, gm)
	e.broadcastToNearby(gm, x, y, -1, network.MsgNpcAppear, n.ToNpcAppear())
	log.Printf("Player %s (level %d) summoned %s", p.Name, p.Level, name)
}

// dismissSummon removes a summoned creature (time up, or its master left).
func (e *Engine) dismissSummon(n *npc.NPC, gm *mapdata.GameMap) {
	gm.ClearOwner(n.X, n.Y)
	e.npcs.Delete(n.ObjectID)
	e.broadcastToNearby(gm, n.X, n.Y, -1, network.MsgNpcDisappear, &pb.NpcDisappear{ObjectId: n.ObjectID})
}

// processSummonTick runs a summoned creature's own AI: stay with the master, pick
// fights with monsters near them, and vanish when its time is up. It returns false
// when the creature is fighting, so the normal chase/attack states take over (with
// its "spawn point" kept on the master, the usual leash keeps it close).
func (e *Engine) processSummonTick(n *npc.NPC, now time.Time, gm *mapdata.GameMap) bool {
	master := e.getPlayerByID(n.MasterID)
	if master == nil || master.HP <= 0 || master.MapName != n.MapName || now.After(n.SummonExpires) {
		e.dismissSummon(n, gm)
		return true
	}
	n.SpawnX, n.SpawnY = master.X, master.Y

	if n.TargetID != 0 {
		tx, ty, ok := e.npcTargetPosition(n)
		if !ok || max(abs(tx-master.X), abs(ty-master.Y)) > 12 {
			e.npcDropTarget(n, gm)
		}
	}
	if n.TargetID == 0 {
		if enemy, _ := e.findNearestEnemyNPC(n, max(n.Type.AggroRange, 6)); enemy != nil {
			n.TargetID = enemy.ObjectID
			n.State = npc.StateChase
		}
	}
	if n.TargetID != 0 {
		return false
	}

	// Nothing to fight: follow the master, keeping a step or two away.
	n.State = npc.StateIdle
	if n.DistanceTo(master.X, master.Y) <= 2 {
		e.npcStopMoving(n, gm)
		return true
	}
	if now.Sub(n.LastMoveTime) < npcStepInterval(n) {
		return true
	}
	dir := n.DirectionTo(master.X, master.Y)
	if nx, ny := n.X+dirDX[dir], n.Y+dirDY[dir]; gm.IsWalkable(nx, ny) {
		e.moveNPC(n, gm, nx, ny, dir)
	}
	n.LastMoveTime = now
	return true
}
