package game

import (
	"math/rand"
	"strings"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

// npcStepToward moves an NPC one tile towards (tx, ty). When the straight step
// is blocked it turns, always to its own side (n.Turn), one direction at a
// time until a step is free, like the original's cGetNextMoveDir. A fixed side
// makes it follow a wall round to the end instead of stepping up, down, up
// beside it. It reports whether it moved.
func (e *Engine) npcStepToward(n *npc.NPC, gm *mapdata.GameMap, tx, ty int) bool {
	if (tx == n.X && ty == n.Y) || time.Now().Before(n.HeldUntil) {
		return false
	}
	dir := n.DirectionTo(tx, ty)
	turn := n.Turn
	if turn == 0 {
		turn = 1
	}
	for i := 0; i < 8; i++ {
		d := (dir-1+i*turn+16)%8 + 1
		nx, ny := n.X+dirDX[d], n.Y+dirDY[d]
		if gm.IsWalkable(nx, ny) {
			e.moveNPC(n, gm, nx, ny, d)
			return true
		}
	}
	return false
}

// npcPickWanderDest chooses a walkable spot in the NPC's wander area, a few tiles
// from where it stands.
func npcPickWanderDest(n *npc.NPC, gm *mapdata.GameMap) bool {
	area := n.Area
	if !area.Set {
		r := n.Type.WanderRange
		if r <= 0 {
			return false
		}
		area = npc.Rect{X1: n.SpawnX - r, Y1: n.SpawnY - r, X2: n.SpawnX + r, Y2: n.SpawnY + r, Set: true}
	}
	for try := 0; try < 10; try++ {
		x := area.X1 + rand.Intn(area.X2-area.X1+1)
		y := area.Y1 + rand.Intn(area.Y2-area.Y1+1)
		if n.DistanceTo(x, y) >= 2 && gm.IsWalkable(x, y) {
			n.WanderDestX, n.WanderDestY, n.HasWanderDest = x, y, true
			return true
		}
	}
	return false
}

// npcWanderStep takes one wandering step. It returns false when the NPC is done
// walking for now (arrived, or stuck) and should stand idle.
func (e *Engine) npcWanderStep(n *npc.NPC, gm *mapdata.GameMap) bool {
	if !n.InHomeArea(n.X, n.Y) {
		// Strayed after a chase: head home
		n.WanderDestX, n.WanderDestY, n.HasWanderDest = n.SpawnX, n.SpawnY, true
	} else if !n.HasWanderDest && !npcPickWanderDest(n, gm) {
		return false
	}
	if n.X == n.WanderDestX && n.Y == n.WanderDestY {
		n.HasWanderDest = false
		return false
	}
	if !e.npcStepToward(n, gm, n.WanderDestX, n.WanderDestY) {
		n.HasWanderDest = false // boxed in: pick somewhere else next time
		return false
	}
	if n.X == n.WanderDestX && n.Y == n.WanderDestY {
		n.HasWanderDest = false
		return false // arrived: the stop goes out after this step and the client finishes it
	}
	return true
}

// npcStepInterval is how often an NPC takes a step: its NPC.cfg action time
// plus 0-300 ms of its own, as the original added rand()%300 per NPC, so a
// pack of monsters doesn't move in lockstep.
func npcStepInterval(n *npc.NPC) time.Duration {
	return time.Duration(n.Type.MoveSpeed+int(n.ObjectID%300)) * time.Millisecond
}

// pointInArea picks a free walkable tile inside a spawn rectangle (the original
// placed generator mobs at random inside it), or the nearest one to its centre.
func (e *Engine) pointInArea(gm *mapdata.GameMap, r npc.Rect) (int, int) {
	for try := 0; try < 30; try++ {
		x := r.X1 + rand.Intn(r.X2-r.X1+1)
		y := r.Y1 + rand.Intn(r.Y2-r.Y1+1)
		if gm.IsWalkable(x, y) {
			return x, y
		}
	}
	return e.findWalkable(gm, (r.X1+r.X2)/2, (r.Y1+r.Y2)/2)
}

// mapNamed finds a loaded map by name, ignoring case (map files keep their
// capitals: "Cmdhall_1.amd" is "Cmdhall_1").
func (e *Engine) mapNamed(name string) (*mapdata.GameMap, bool) {
	if gm, ok := e.maps[name]; ok {
		return gm, true
	}
	for n, gm := range e.maps {
		if strings.EqualFold(n, name) {
			return gm, true
		}
	}
	return nil, false
}

// teleportAt is the door on a tile the map marks as a teleport. Our maps mark
// some doorways a tile or two wider than the original's teleport-loc list, so
// a marked tile without its own entry uses the nearest door within 2 tiles.
func (e *Engine) teleportAt(mapName string, x, y int) (mapdata.TeleportDest, bool) {
	if dest, ok := e.teleports[mapdata.TeleportKey(mapName, x, y)]; ok {
		return dest, true
	}
	for r := 1; r <= 2; r++ {
		for dy := -r; dy <= r; dy++ {
			for dx := -r; dx <= r; dx++ {
				if max(abs(dx), abs(dy)) != r {
					continue
				}
				if dest, ok := e.teleports[mapdata.TeleportKey(mapName, x+dx, y+dy)]; ok {
					return dest, true
				}
			}
		}
	}
	return mapdata.TeleportDest{}, false
}
