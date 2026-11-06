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
// each map with random-mob-generator = 1 <level> that holds fewer than
// maximum-object - 30 monsters gets one more group: a monster rolled from the
// level's table (random_mobs_table.go) somewhere on the map outside its
// npc-avoidrect areas, plus 0-4 of the same next to it. They wander the whole
// map and are removed when they die; the generator tops the map up again.

const randomMobInterval = 600 * time.Millisecond

// runRandomMobs adds one group to every map that has room (processTick).
func (e *Engine) runRandomMobs() {
	if RandomMobDensity <= 0 {
		return
	}
	live := map[string]int{}
	e.npcs.Range(func(_, v any) bool {
		if n, ok := v.(*npc.NPC); ok && n.State != npc.StateDead {
			live[strings.ToLower(n.MapName)]++
		}
		return true
	})
	for _, gm := range e.maps {
		c := mapdata.Configs().Get(gm.Name)
		if c == nil || !c.RandomMobs.On {
			continue
		}
		limit := int(float64(c.MaxObjects-30) * RandomMobDensity)
		if live[strings.ToLower(gm.Name)] < limit {
			e.spawnRandomGroup(gm, c)
		}
	}
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
	e.npcs.Store(objectID, n)
	gm.SetOwner(x, y, objectID)
	e.broadcastToNearby(gm, x, y, -1, network.MsgNpcAppear, n.ToNpcAppear())
	return true
}
