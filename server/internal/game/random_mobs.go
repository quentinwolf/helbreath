package game

import (
	"math/rand"
	"strings"
	"time"

	"github.com/juanrossi/hbonline/server/internal/mapdata"
	"github.com/juanrossi/hbonline/server/internal/network"
	"github.com/juanrossi/hbonline/server/internal/npc"
)

// The random mob generator (original Game.cpp MobGenerator): every 600 ms,
// each map with random-mob-generator = 1 <level> that has room gets one more
// monster rolled from the level's table (random_mobs_table.go), somewhere on
// the map outside its npc-avoidrect areas. They wander the whole map and are
// removed when they die; the generator tops the map up again.
//
// Changed from the original (Zac's design): the original filled a map up to
// maximum-object - 30 with random monsters in packs of up to 5. Here random
// monsters have their own cap (RandomMobCap: random-mob-max, or RandomMobShare
// of maximum-object), only monsters count toward maximum-object, so spot
// generators always get their share, and they come one at a time unless the
// map sets random-mob-groups = 1.

const randomMobInterval = 600 * time.Millisecond

// runRandomMobs adds one group to every map that has room (processTick). A map
// over its cap (the map editor lowered it, or turned the generator off) loses
// one idle random monster nobody is near instead.
func (e *Engine) runRandomMobs() {
	random := map[string]int{}     // live random monsters per map
	monsters := map[string]int{}   // all live wild monsters per map
	spare := map[string]*npc.NPC{} // a random monster that can go unseen
	e.npcs.Range(func(_, v any) bool {
		n, ok := v.(*npc.NPC)
		if !ok || n.State == npc.StateDead || n.Type.Side != npc.SideMonster || n.MasterID != 0 {
			return true
		}
		m := strings.ToLower(n.MapName)
		monsters[m]++
		if n.RandomMob {
			random[m]++
			if spare[m] == nil && n.TargetID == 0 {
				spare[m] = n
			}
		}
		return true
	})
	for _, gm := range e.maps {
		m := strings.ToLower(gm.Name)
		c := mapdata.Configs().Get(gm.Name)
		limit := 0
		if c != nil && c.RandomMobs.On {
			limit = RandomMobCap(c)
		}
		if random[m] > limit {
			if n := spare[m]; n != nil && len(gm.GetNearbyPlayerIDs(n.X, n.Y)) == 0 {
				gm.ClearOwner(n.X, n.Y)
				e.npcs.Delete(n.ObjectID)
			}
			continue
		}
		if c != nil && c.RandomMobs.On && random[m] < limit && (c.MaxObjects <= 0 || monsters[m] < c.MaxObjects) {
			e.spawnRandomGroup(gm, c)
		}
	}
}

// RandomMobCap is the most random monsters a map has at once: its
// random-mob-max, or RandomMobShare of its maximum-object.
func RandomMobCap(c *mapdata.MapConfig) int {
	if c.RandomMobs.MaxSet {
		return c.RandomMobs.Max
	}
	return int(float64(c.MaxObjects) * RandomMobShare)
}

// spawnRandomGroup adds one rolled monster and its followers; how many were made.
func (e *Engine) spawnRandomGroup(gm *mapdata.GameMap, c *mapdata.MapConfig) int {
	code, _ := randomMobCode(c.RandomMobs.Level, iDice)
	name, ok := randomMobNames[code]
	if !ok {
		name = "Orc" // the original's default
	}
	id, err := npc.TypeForConfigName(name)
	if err != nil {
		return 0 // a monster this port doesn't have (Barlog, the soldiers)
	}
	t := npc.NpcTypes[id]
	x, y, ok := randomMobSpot(gm, c)
	if !ok {
		return 0
	}
	made := 0
	if e.spawnRandomMob(gm, t, x, y) {
		made++
	}
	if !c.RandomMobs.Groups {
		return made // one at a time (the original's packs only with random-mob-groups = 1)
	}
	for i := randomMobFollowers(code, iDice); i > 0; i-- {
		fx, fy := x+rand.Intn(3)-1, y+rand.Intn(3)-1
		if !gm.IsWalkable(fx, fy) {
			fx, fy = e.findWalkable(gm, x, y)
		}
		if e.spawnRandomMob(gm, t, fx, fy) {
			made++
		}
	}
	return made
}

// randomMobSpot is where a group starts: the original's 31 tries at a random
// tile 15 or more from the left/top edge and 35 from the right/bottom, outside
// every npc-avoidrect. We also want a walkable tile.
func randomMobSpot(gm *mapdata.GameMap, c *mapdata.MapConfig) (int, int, bool) {
	if gm.Width < 51 || gm.Height < 51 {
		return 0, 0, false
	}
	for try := 0; try <= 30; try++ {
		x := rand.Intn(gm.Width-50) + 15
		y := rand.Intn(gm.Height-50) + 15
		if !gm.IsWalkable(x, y) || c.InNpcAvoid(x, y) {
			continue
		}
		return x, y, true
	}
	return 0, 0, false
}

func (e *Engine) spawnRandomMob(gm *mapdata.GameMap, t *npc.NpcType, x, y int) bool {
	if !gm.IsWalkable(x, y) {
		return false
	}
	objectID := e.nextObjID.Add(1)
	n := npc.NewNPC(objectID, t, gm.Name, x, y)
	n.RandomMob = true
	// Roam the whole map like the original (DEF_MOVETYPE_RANDOM picks each
	// destination 15+ tiles from the left/top edge, 35 from the right/bottom),
	// not 8 tiles round the spawn point: that kept every pack on one spot
	n.Area = npc.Rect{X1: 15, Y1: 15, X2: gm.Width - 36, Y2: gm.Height - 36, Set: true}
	e.npcs.Store(objectID, n)
	gm.SetOwner(x, y, objectID)
	e.broadcastAroundNPC(gm, x, y, -1, network.MsgNpcAppear, n.ToNpcAppear())
	return true
}
