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
	// Random-generator mobs roam the whole map but keep out of its
	// npc-avoidrect areas (the town), as the original's DEF_MOVETYPE_RANDOM
	var avoid *mapdata.MapConfig
	if n.RandomMob {
		avoid = mapdata.Configs().Get(gm.Name)
	}
	for try := 0; try < 10; try++ {
		x := area.X1 + rand.Intn(area.X2-area.X1+1)
		y := area.Y1 + rand.Intn(area.Y2-area.Y1+1)
		if avoid != nil && avoid.InNpcAvoid(x, y) {
			continue
		}
		if n.DistanceTo(x, y) >= 2 && gm.IsWalkable(x, y) {
			setWanderDest(n, x, y)
			return true
		}
	}
	return false
}

func setWanderDest(n *npc.NPC, x, y int) {
	n.WanderDestX, n.WanderDestY, n.HasWanderDest = x, y, true
	n.WanderBest, n.WanderStall = -1, 0
}

// wanderStallSteps: a wandering NPC that takes this many steps without getting
// any closer to its destination gives up on it and picks another. Stepping
// round a wall toward a spot behind it otherwise turns into a back-and-forth
// that never ends (the original's greedy stepping has the same trap).
const wanderStallSteps = 8

// npcWanderStep takes one wandering step. It returns false when the NPC is done
// walking for now (arrived, or stuck) and should stand idle.
func (e *Engine) npcWanderStep(n *npc.NPC, gm *mapdata.GameMap) bool {
	if !n.HasWanderDest {
		// A spot in the wander area; one that strayed after a chase heads back
		// into it (to the spawn point if no spot can be found)
		if !npcPickWanderDest(n, gm) {
			if n.InHomeArea(n.X, n.Y) {
				return false
			}
			setWanderDest(n, n.SpawnX, n.SpawnY)
		}
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
	// Progress check: squared distance, so moving diagonally along a wall that
	// still gains ground counts as getting closer
	dx, dy := n.WanderDestX-n.X, n.WanderDestY-n.Y
	if d := dx*dx + dy*dy; n.WanderBest < 0 || d < n.WanderBest {
		n.WanderBest, n.WanderStall = d, 0
	} else if n.WanderStall++; n.WanderStall >= wanderStallSteps {
		n.HasWanderDest = false // stuck behind something: try somewhere else
		return false
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

