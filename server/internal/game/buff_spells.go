package game

import (
	"math/rand"

	"github.com/juanrossi/hbonline/server/internal/items"
	"github.com/juanrossi/hbonline/server/internal/magic"
	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/player"
)

// Buffs and Create Food. The original cast Defense Shield, Protection From
// Arrow/Magic and Berserk on whoever stood on the clicked tile; here they go
// on the clicked player if the caster may buff them, otherwise on the caster.

// Original item IDs Create Food makes (the original rolled Meat or Baguette).
const (
	itemBaguette = 98
	itemMeat     = 99
)

// targetedBuff reports whether a buff can be put on another player.
func targetedBuff(spell *magic.SpellDef) bool {
	return spell.CfgType == magic.CfgTypeProtect || spell.CfgType == magic.CfgTypeBerserk
}

// canBuff is who may put a buff on whom (Zac's rule): a GM anyone; everyone
// else themselves and their own side (Aresden, Elvine, or other travellers).
func canBuff(caster, t *player.Player) bool {
	return caster == t || caster.AdminLevel > 0 || sameSide(caster, t)
}

// castBuff buffs the clicked player (when allowed) or the caster.
func (e *Engine) castBuff(p *player.Player, spell *magic.SpellDef, targetID int32, gm *mapdata.GameMap) {
	target := p
	if targetedBuff(spell) && targetID != 0 && targetID != p.ObjectID {
		if t := e.getPlayerByID(targetID); t != nil && t.HP > 0 && t.MapName == p.MapName {
			if !canBuff(p, t) {
				e.sendNotification(p, "You can only cast that on yourself or your own side", 2)
				return
			}
			target = t
		}
	}
	e.buffPlayer(p, target, spell, gm)
}

// createFoodCheck is why Create Food can't land on a tile ("" = it can).
func createFoodCheck(p *player.Player, gm *mapdata.GameMap, tx, ty int) string {
	if max(abs(tx-p.X), abs(ty-p.Y)) > magic.MaxCastRange {
		return "Target out of range"
	}
	if tx < 0 || ty < 0 || tx >= gm.Width || ty >= gm.Height || !gm.Tiles[ty][tx].Walkable {
		return "You can't make food there"
	}
	return ""
}

// castCreateFood drops a Meat or a Baguette on the clicked tile, as the original.
func (e *Engine) castCreateFood(p *player.Player, spell *magic.SpellDef, tx, ty int, gm *mapdata.GameMap) {
	e.broadcastSpellEffect(p, spell, 0, p.X, p.Y, tx, ty, 0, 0, false, gm)
	id := itemMeat
	if rand.Intn(2) == 0 {
		id = itemBaguette
	}
	if def := items.GetItemDef(id); def != nil {
		e.dropGroundItem(items.NewItem(def, 1), p.MapName, tx, ty, gm)
	}
}
