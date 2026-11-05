package game

import (
	"math/rand"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

// npcStepToward moves an NPC one tile towards (tx, ty). When the straight step
// is blocked it tries the two neighbouring directions, then the next two out,
// so it walks around a pillar instead of freezing. It reports whether it moved.
func (e *Engine) npcStepToward(n *npc.NPC, gm *mapdata.GameMap, tx, ty int) bool {
	dir := n.DirectionTo(tx, ty)
	if tx == n.X && ty == n.Y {
		return false
	}
	// Try the closer side first, so detours bend towards the target
	first, second := 1, -1
	if rand.Intn(2) == 0 {
		first, second = -1, 1
	}
	for _, turn := range []int{0, first, second, 2 * first, 2 * second} {
		d := (dir-1+turn+8)%8 + 1
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
	r := n.Type.WanderRange
	if r <= 0 {
		return false
	}
	for try := 0; try < 10; try++ {
		x := n.SpawnX + rand.Intn(2*r+1) - r
		y := n.SpawnY + rand.Intn(2*r+1) - r
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
	if n.DistanceTo(n.SpawnX, n.SpawnY) > n.Type.WanderRange {
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
