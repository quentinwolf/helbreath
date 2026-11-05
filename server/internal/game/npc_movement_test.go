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
