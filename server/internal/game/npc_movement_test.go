package game

import "testing"

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
