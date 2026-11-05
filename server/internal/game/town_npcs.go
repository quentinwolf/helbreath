package game

import (
	"fmt"
	"strings"

	"github.com/juanrossi/hbonline/server/internal/npc"
	"github.com/juanrossi/hbonline/server/internal/player"
)

// Town officers that open a menu instead of a shop.
const (
	npcCityHallOfficer  = 25 // William: citizenship
	npcGuildHallOfficer = 26 // Kennedy: guilds
)

// notifyOpenMenu is the Notification type that opens an NPC's menu on the
// client: "cityhall:<side>" (side = the town this City Hall belongs to) or
// "guildhall".
const notifyOpenMenu = 12

// townSide is the town a map belongs to (1 Aresden, 2 Elvine, 0 neither), like
// the original's per-map location name. Town buildings end in _1 / _2.
func townSide(mapName string) int {
	m := strings.ToLower(mapName)
	switch {
	case m == "aresden" || strings.HasPrefix(m, "are") || strings.HasSuffix(m, "_1") || strings.HasSuffix(m, "_1f"):
		return 1
	case m == "elvine" || strings.HasPrefix(m, "elv") || strings.HasSuffix(m, "_2") || strings.HasSuffix(m, "_2f"):
		return 2
	}
	return 0
}

// nearNPCType reports whether an NPC of the given type is within dist tiles.
func (e *Engine) nearNPCType(p *player.Player, typeID, dist int) bool {
	found := false
	e.npcs.Range(func(_, value any) bool {
		n := value.(*npc.NPC)
		if n.Type.ID == typeID && n.MapName == p.MapName && n.DistanceTo(p.X, p.Y) <= dist {
			found = true
			return false
		}
		return true
	})
	return found
}

// openOfficerMenu opens William's or Kennedy's menu; false for other NPCs.
func (e *Engine) openOfficerMenu(p *player.Player, n *npc.NPC) bool {
	switch n.Type.ID {
	case npcCityHallOfficer:
		e.sendNotification(p, fmt.Sprintf("cityhall:%d", townSide(n.MapName)), notifyOpenMenu)
	case npcGuildHallOfficer:
		e.sendNotification(p, "guildhall", notifyOpenMenu)
	default:
		return false
	}
	return true
}
