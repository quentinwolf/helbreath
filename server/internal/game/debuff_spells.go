package game

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/juanrossi/hbonline/server/internal/items"
	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
	pb "github.com/juanrossi/hbonline/server/pkg/proto"
)

// Debuffs ported from the original server's magic handler (Server.cpp
// ~17850-19830): Stamina Drain, Hold Person / Paralyze, Possession, and the
// Confuse family (Confuse Language, Confusion, Illusion, Illusion Movement,
// and their Mass versions).

// notifyStatus tells the affected player's client a status started or ended,
// so it can apply it locally as the original client did: "hold:<level>",
// "confuse:<level>:<caster object id>", "hold-off", "confuse-off".
const notifyStatus = 13

// The original client's messages (NOTIFYMSG_MAGICEFFECT_ON*).
var statusOnText = map[string]string{
	"hold:1":    "You were bound by a Hold Person spell! Unable to move!",
	"hold:2":    "You were bound by a Paralysis spell! Unable to move!",
	"confuse:1": "No one understands you because of language confusion magic!",
	"confuse:2": "Confusion magic cast: impossible to tell friend from foe.",
	"confuse:3": "Illusion magic cast: impossible to tell who is who!",
	"confuse:4": "You are thrown into confusion and your feet go the wrong way!",
}

// castOriginalDebuff handles the debuffs above; false for any other spell.
func (e *Engine) castOriginalDebuff(p *player.Player, spell *magic.SpellDef, tx, ty int, targetID int32, gm *mapdata.GameMap) bool {
	switch spell.CfgType {
	case magic.CfgTypeSPDownArea:
		e.castStaminaDrain(p, spell, tx, ty, gm)
	case magic.CfgTypeHold:
		e.castHold(p, spell, targetID, gm)
	case magic.CfgTypePossession:
		e.castPossession(p, spell, tx, ty, gm)
	case magic.CfgTypeConfuse:
		e.castConfuse(p, spell, tx, ty, gm)
	default:
		return false
	}
	return true
}

// resistRoll is the magic resistance roll (a variable so tests can fix it).
var resistRoll = magic.CheckMagicResistance

// resists is the target's magic resistance roll (GMs in god mode always resist).
func resists(caster, t *player.Player, spell *magic.SpellDef) bool {
	return t.GodMode || resistRoll(caster.EffectiveMAG(), t.EffectiveINT(), max(spell.EffectLevel, 1))
}

// playersInBox lists the players the caster may hit inside a box around (tx,ty).
func (e *Engine) playersInBox(p *player.Player, tx, ty, ax, ay int) []*player.Player {
	var out []*player.Player
	e.players.Range(func(_, v any) bool {
		t := v.(*player.Player)
		if spellCanHitPlayer(p, t) && abs(t.X-tx) <= ax && abs(t.Y-ty) <= ay {
			out = append(out, t)
		}
		return true
	})
	return out
}

// castStaminaDrain: SPDice off whoever is on the clicked tile, AreaSPDice off
// everyone in the box (the clicked player too), each after a resistance roll.
// Monsters have no stamina (the original did nothing to NPCs either).
func (e *Engine) castStaminaDrain(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	e.broadcastSpellEffect(p, spell, 0, p.X, p.Y, tx, ty, 0, 0, false, gm)
	for _, t := range e.playersInBox(p, tx, ty, spell.AreaX, spell.AreaY) {
		drain := 0
		if t.X == tx && t.Y == ty && !resists(p, t, spell) {
			drain += rollDice(spell.SPDice)
		}
		if !resists(p, t, spell) {
			drain += rollDice(spell.AreaSPDice)
		}
		if drain <= 0 || t.SP <= 0 {
			continue
		}
		t.SP = max(t.SP-drain, 0)
		e.cancelLogoutOnDamage(t)
		e.sendStatUpdate(t)
		e.sendNotification(t, fmt.Sprintf("%s drained %d of your stamina", p.Name, drain), 2)
	}
}

// castHold: Hold Person (level 1) / Paralyze (2) stops the target walking or
// running for the spell's duration; spell damage breaks it (spellHurtPlayer).
// Not on your own side, not on someone already held, and the target may resist.
func (e *Engine) castHold(p *player.Player, spell *magic.SpellDef, targetID int32, gm *mapdata.GameMap) {
	duration := time.Duration(spell.Duration) * time.Second
	if val, ok := e.npcs.Load(targetID); ok {
		n := val.(*npc.NPC)
		e.broadcastSpellEffect(p, spell, targetID, p.X, p.Y, n.X, n.Y, 0, 0, false, gm)
		if !n.IsAlive() || time.Now().Before(n.HeldUntil) || !e.playerCanHurtNPC(p, n) {
			return
		}
		if resistRoll(p.EffectiveMAG(), n.Type.INT, max(spell.EffectLevel, 1)) {
			e.sendNotification(p, n.Type.Name+" resisted the spell", 2)
			return
		}
		n.HeldUntil = time.Now().Add(duration)
		e.npcStopMoving(n, gm)
		if n.TargetID == 0 {
			n.TargetID = p.ObjectID
			n.State = npc.StateChase
		}
		return
	}
	t := e.getPlayerByID(targetID)
	if t == nil {
		return
	}
	e.broadcastSpellEffect(p, spell, targetID, p.X, p.Y, t.X, t.Y, 0, 0, false, gm)
	if !spellCanHitPlayer(p, t) || t.Effects == nil || t.Effects.HasEffect(magic.EffectHold) {
		return
	}
	if resists(p, t, spell) {
		e.sendNotification(p, t.Name+" resisted the spell", 2)
		e.sendNotification(t, "You resisted "+spell.Name, 3)
		return
	}
	level := 1
	if spell.EffectLevel == 2 {
		level = 2
	}
	t.Effects.AddEffect(&magic.ActiveEffect{
		Type: magic.EffectHold, Level: level, ExpiresAt: time.Now().Add(duration), SourceID: p.ObjectID,
	})
	e.cancelLogoutOnDamage(t)
	e.sendStatusOn(t, fmt.Sprintf("hold:%d", level))
}

// castPossession pulls an item lying on an empty tile into the caster's bag.
// Citizens only, as in the original.
func (e *Engine) castPossession(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	e.broadcastSpellEffect(p, spell, 0, p.X, p.Y, tx, ty, 0, 0, false, gm)
	if p.Side == 0 {
		e.sendNotification(p, "Only citizens can use Possession", 2)
		return
	}
	if e.livingObjectAt(p.MapName, tx, ty, p.ObjectID) != 0 {
		return // someone is standing on it
	}
	var gid int32
	var found *items.GroundItem
	e.groundItems.Range(func(k, v any) bool {
		gi := v.(*items.GroundItem)
		if gi.MapName == p.MapName && gi.X == tx && gi.Y == ty {
			gid, found = k.(int32), gi
			return false
		}
		return true
	})
	if found == nil {
		e.sendNotification(p, "There is nothing there to take", 2)
		return
	}
	e.takeGroundItem(p, gid, found)
}

// castConfuse: everyone on the other side inside the box who fails to resist
// and isn't already confused gets the spell's confusion (sValue4) for its
// duration. Illusion Movement fails while the caster is invisible.
func (e *Engine) castConfuse(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	e.broadcastSpellEffect(p, spell, 0, p.X, p.Y, tx, ty, 0, 0, false, gm)
	level := spell.EffectLevel
	if level == magic.ConfuseIllusionMovement && p.Effects != nil && p.Effects.HasEffect(magic.EffectInvisibility) {
		return
	}
	duration := time.Duration(spell.Duration) * time.Second
	for _, t := range e.playersInBox(p, tx, ty, spell.AreaX, spell.AreaY) {
		if t.Effects == nil || t.Effects.HasEffect(magic.EffectConfusion) {
			continue
		}
		if resists(p, t, spell) {
			e.sendNotification(t, "You resisted "+spell.Name, 3)
			continue
		}
		t.Effects.AddEffect(&magic.ActiveEffect{
			Type: magic.EffectConfusion, Level: level, ExpiresAt: time.Now().Add(duration), SourceID: p.ObjectID,
		})
		e.cancelLogoutOnDamage(t)
		e.sendStatusOn(t, fmt.Sprintf("confuse:%d:%d", level, p.ObjectID))
	}
}

// sendStatusOn tells a player's client a status started, with the original message.
func (e *Engine) sendStatusOn(t *player.Player, status string) {
	key := status
	if parts := strings.SplitN(status, ":", 3); len(parts) >= 2 {
		key = parts[0] + ":" + parts[1]
	}
	if text, ok := statusOnText[key]; ok {
		e.sendNotification(t, text, 2)
	}
	e.sendNotification(t, status, notifyStatus)
}

// sendStatusOff tells the client a Hold or Confuse status ended.
func (e *Engine) sendStatusOff(t *player.Player, eff magic.EffectType) {
	switch eff {
	case magic.EffectHold:
		e.sendNotification(t, "hold-off", notifyStatus)
	case magic.EffectConfusion:
		e.sendNotification(t, "confuse-off", notifyStatus)
	}
}

// Confuse Language (Zac's design; the original scrambled the confused player's
// own speech instead): a confused player can't understand anyone else. Every
// chat line reaching them comes out as gibberish, except their own, a GM's,
// and another confused player's (they share the confused tongue).

// confusedLanguage reports whether a player is under Confuse Language.
func confusedLanguage(p *player.Player) bool {
	return p != nil && p.Effects != nil && p.Effects.GetEffectLevel(magic.EffectConfusion) == magic.ConfuseLanguage
}

// sendChat delivers one chat line to one reader, scrambled if they can't understand it.
func (e *Engine) sendChat(to *player.Player, msg *pb.ChatMessage, data []byte) {
	var sender *player.Player
	if to.ObjectID != msg.ObjectId && confusedLanguage(to) {
		if sender = e.getPlayerByID(msg.ObjectId); sender == nil {
			sender = e.getPlayerByName(msg.SenderName)
		}
	}
	if understands(to, sender, msg.ObjectId) {
		to.Send(data)
		return
	}
	garbled := proto.Clone(msg).(*pb.ChatMessage)
	garbled.Message = gibberish(msg.Message)
	if out, err := network.Encode(network.MsgChatMessage, garbled); err == nil {
		to.Send(out)
	}
}

// understands reports whether a reader can read a line from sender (nil = unknown,
// e.g. a system line).
func understands(reader, sender *player.Player, senderID int32) bool {
	if reader.ObjectID == senderID || !confusedLanguage(reader) {
		return true
	}
	return sender != nil && (sender.AdminLevel > 0 || confusedLanguage(sender))
}

// gibberish turns a line into random Korean syllables, one per two letters,
// keeping the word breaks (as the original's Confuse Language did).
func gibberish(msg string) string {
	syllables := []rune("갑뷁쀍똠휶괎솗맗")
	var b strings.Builder
	for _, word := range strings.Fields(msg) {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		for i := 0; i < (len([]rune(word))+1)/2; i++ {
			b.WriteRune(syllables[rand.Intn(len(syllables))])
		}
	}
	return b.String()
}
