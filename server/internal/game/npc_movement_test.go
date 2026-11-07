package game

import (
	"testing"
	"time"

	"github.com/juanrossi/hbonline/server/internal/npc"
)

func TestWanderingWalksSteadilyToItsSpot(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	n := makeTestNPC(1013) // a guard with a big wander area
	n.MapName = "test"
	n.WanderDestX, n.WanderDestY, n.HasWanderDest = n.X+6, n.Y+3, true

	prev := n.DistanceTo(n.WanderDestX, n.WanderDestY)
	for i := 0; i < 20 && n.HasWanderDest; i++ {
		if !e.npcWanderStep(n, gm) {
			break
		}
		d := n.DistanceTo(n.WanderDestX, n.WanderDestY)
		if d >= prev {
			t.Fatalf("step %d went sideways or back (distance %d -> %d)", i, prev, d)
		}
		prev = d
	}
	if n.X != 56 || n.Y != 53 {
		t.Errorf("should arrive at its spot, got %d,%d", n.X, n.Y)
	}
}

func TestStepTowardGoesAroundABlock(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	n := makeTestNPC(1013)
	n.MapName = "test"
	gm.SetOwner(51, 50, 999) // someone stands in the way east
	if !e.npcStepToward(n, gm, 55, 50) {
		t.Fatal("should step around the blocked tile")
	}
	if n.X != 51 || (n.Y != 49 && n.Y != 51) {
		t.Errorf("should sidestep diagonally, got %d,%d", n.X, n.Y)
	}
}

func TestShopkeepersStayPutFacingTheRoom(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	for i := 0; i < 50; i++ {
		n := npc.NewNPC(int32(5000+i), npc.NpcTypes[20], "test", 50, 50) // Howard
		if n.Direction < 4 || n.Direction > 6 {
			t.Fatalf("Howard faces %d, want 4-6 (down-right, down, down-left)", n.Direction)
		}
		now := time.Now()
		for tick := 0; tick < 20; tick++ {
			n.NextThinkTime = time.Time{}
			e.processNPCTick(n, now.Add(time.Duration(tick)*time.Minute))
		}
		if n.X != 50 || n.Y != 50 || n.State != npc.StateIdle {
			t.Fatalf("Howard moved to %d,%d (state %v)", n.X, n.Y, n.State)
		}
	}
}

func TestTownWalkersStepSlowly(t *testing.T) {
	for _, id := range []int{67, 68, 69} {
		n := npc.NewNPC(1, npc.NpcTypes[id], "test", 50, 50)
		if got := npcStepInterval(n); got < 5*time.Second {
			t.Errorf("%s steps every %v, want the NPC.cfg action time (5 s or more)", n.Type.Name, got)
		}
	}
}

func TestStepTowardFollowsAWallWithoutFlipping(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(100, 100)
	e.maps["test"] = gm
	for y := 40; y <= 60; y++ {
		gm.SetOwner(51, y, 999) // a wall east of the NPC, x=51, y 40..60
	}
	n := makeTestNPC(1013)
	n.MapName = "test"
	n.Turn = 1
	lastDY := 0
	for i := 0; i < 30 && n.X <= 50; i++ {
		y := n.Y
		if !e.npcStepToward(n, gm, 55, 50) {
			t.Fatal("stuck against the wall")
		}
		dy := n.Y - y
		if lastDY != 0 && dy != 0 && dy != lastDY {
			t.Fatalf("step %d turned back along the wall (dy %d then %d)", i, lastDY, dy)
		}
		if dy != 0 {
			lastDY = dy
		}
	}
	if n.X <= 50 {
		t.Errorf("never got round the wall, at %d,%d", n.X, n.Y)
	}
}

// A wanderer heading for a spot behind a wall used to step back and forth along
// the wall for good; after a few steps without getting closer it picks another spot.
func TestWandererBehindWallPicksNewDestination(t *testing.T) {
	for _, turn := range []int{1, -1} {
		e := makeTestEngine()
		gm := makeTestGameMap(60, 60)
		e.maps["test"] = gm
		// A pocket open to the west: a wall at x = 30 (rows 25-35) and walls along
		// rows 25 and 35 from x = 22. The NPC is inside, its destination straight
		// east behind the wall: it slides into a corner and steps in and out of it.
		for y := 25; y <= 35; y++ {
			gm.Tiles[y][30].Walkable = false
		}
		for x := 22; x <= 30; x++ {
			gm.Tiles[25][x].Walkable = false
			gm.Tiles[35][x].Walkable = false
		}
		n := npc.NewNPC(1, npc.NpcTypes[55], "test", 28, 30)
		n.Turn = turn
		n.Area = npc.Rect{X1: 1, Y1: 1, X2: 58, Y2: 58, Set: true}
		e.npcs.Store(n.ObjectID, n)
		setWanderDest(n, 33, 30)

		gaveUp := false
		for i := 0; i < 100 && !gaveUp; i++ {
			gaveUp = !e.npcWanderStep(n, gm)
		}
		if !gaveUp {
			t.Fatalf("turn %d: NPC should give up on a destination it can't get closer to, still at %d,%d", turn, n.X, n.Y)
		}
		if n.HasWanderDest {
			t.Errorf("turn %d: the stuck destination should be dropped", turn)
		}
	}
}

// NPC events reach every player within npcViewRange (40 tiles), the same range
// NPCs are shown in. They used to go to the 3x3 sectors round the NPC only (as
// little as 20 tiles), so an NPC 20-40 tiles off froze on screen, then flew to
// wherever it had walked once it came back in range.
func TestNpcEventsReachPlayersInViewRange(t *testing.T) {
	e := makeTestEngine()
	gm := makeTestGameMap(200, 200)
	e.maps["test"] = gm
	p := makeFullTestPlayer(1, "Watcher")
	p.X, p.Y = 39, 50 // last column of sector 1
	addPlayerToEngine(e, p)
	gm.AddPlayerToSector(p.X, p.Y, p.ObjectID)

	has := func(x, y int) bool {
		for _, v := range e.npcViewers(gm, x, y) {
			if v == p {
				return true
			}
		}
		return false
	}
	if !has(61, 50) { // 22 tiles away, sector 3: outside the old 3x3 sectors
		t.Error("a player 22 tiles from an NPC should get its events")
	}
	if !has(79, 90) { // 40 tiles on both axes
		t.Error("a player exactly 40 tiles away should get its events")
	}
	if has(80, 50) {
		t.Error("a player 41 tiles away is out of view range")
	}
}
