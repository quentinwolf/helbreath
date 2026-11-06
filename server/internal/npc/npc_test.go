package npc

import (
	"strings"
	"testing"
	"time"
)

func TestNewNPC(t *testing.T) {
	npcType := NpcTypes[10] // Slime
	n := NewNPC(100, npcType, "default", 50, 55)

	if n.ObjectID != 100 {
		t.Errorf("Expected ObjectID=100, got %d", n.ObjectID)
	}
	if n.Type.Name != "Slime" {
		t.Errorf("Expected Slime, got %s", n.Type.Name)
	}
	if n.MapName != "default" {
		t.Errorf("Expected default map, got %s", n.MapName)
	}
	if n.X != 50 || n.Y != 55 {
		t.Errorf("Expected (50,55), got (%d,%d)", n.X, n.Y)
	}
	if n.SpawnX != 50 || n.SpawnY != 55 {
		t.Errorf("Spawn should match initial position")
	}
	expectedHP := NpcTypes[10].HP
	if n.HP != expectedHP || n.MaxHP != expectedHP {
		t.Errorf("Expected HP=%d, got %d/%d", expectedHP, n.HP, n.MaxHP)
	}
	if n.State != StateIdle {
		t.Errorf("Expected StateIdle, got %d", n.State)
	}
	if n.Direction < 1 || n.Direction > 8 {
		t.Errorf("Direction should be 1-8, got %d", n.Direction)
	}
}

func TestNPCIsAlive(t *testing.T) {
	n := NewNPC(1, NpcTypes[10], "default", 0, 0)
	if !n.IsAlive() {
		t.Error("New NPC should be alive")
	}

	n.HP = 0
	n.State = StateDead
	if n.IsAlive() {
		t.Error("Dead NPC should not be alive")
	}
}

func TestNPCTakeDamage(t *testing.T) {
	n := NewNPC(1, NpcTypes[10], "default", 0, 0)
	startHP := n.HP

	// Take some damage
	killed := n.TakeDamage(5)
	if killed {
		t.Errorf("5 damage should not kill %dHP Slime", startHP)
	}
	if n.HP != startHP-5 {
		t.Errorf("Expected %d HP, got %d", startHP-5, n.HP)
	}

	// Kill it
	killed = n.TakeDamage(startHP)
	if !killed {
		t.Errorf("%d more damage should kill %dHP Slime", startHP, n.HP)
	}
	if n.HP != 0 {
		t.Errorf("Dead NPC HP should be 0, got %d", n.HP)
	}
	if n.State != StateDead {
		t.Error("Should be in StateDead")
	}
}

func TestNPCRespawn(t *testing.T) {
	n := NewNPC(1, NpcTypes[10], "default", 50, 50)
	n.X = 60
	n.Y = 60
	n.TakeDamage(100) // kill it

	n.Respawn()
	if n.HP != n.MaxHP {
		t.Errorf("Should have full HP after respawn, got %d/%d", n.HP, n.MaxHP)
	}
	if n.X != 50 || n.Y != 50 {
		t.Errorf("Should return to spawn position (%d,%d), got (%d,%d)", n.SpawnX, n.SpawnY, n.X, n.Y)
	}
	if n.State != StateIdle {
		t.Error("Should be idle after respawn")
	}
}

func TestNPCReadyToRespawn(t *testing.T) {
	n := NewNPC(1, NpcTypes[10], "default", 0, 0)
	n.TakeDamage(100)

	if n.ReadyToRespawn() {
		t.Error("Should not be ready to respawn immediately")
	}

	n.DeathTime = time.Now().Add(-20 * time.Second) // pretend died 20s ago
	if !n.ReadyToRespawn() {
		t.Error("Should be ready to respawn after delay")
	}
}

func TestNPCDistanceTo(t *testing.T) {
	n := NewNPC(1, NpcTypes[10], "default", 10, 10)

	// Same position
	if d := n.DistanceTo(10, 10); d != 0 {
		t.Errorf("Same pos should be distance 0, got %d", d)
	}

	// Adjacent
	if d := n.DistanceTo(11, 10); d != 1 {
		t.Errorf("Adjacent should be distance 1, got %d", d)
	}

	// Diagonal
	if d := n.DistanceTo(11, 11); d != 1 {
		t.Errorf("Diagonal adjacent should be Chebyshev distance 1, got %d", d)
	}

	// Far away
	if d := n.DistanceTo(20, 15); d != 10 {
		t.Errorf("Expected Chebyshev distance 10, got %d", d)
	}
}

func TestNPCDirectionTo(t *testing.T) {
	n := NewNPC(1, NpcTypes[10], "default", 10, 10)

	tests := []struct {
		tx, ty int
		dir    int
	}{
		{10, 9, 1},  // N
		{11, 9, 2},  // NE
		{11, 10, 3}, // E
		{11, 11, 4}, // SE
		{10, 11, 5}, // S
		{9, 11, 6},  // SW
		{9, 10, 7},  // W
		{9, 9, 8},   // NW
	}

	for _, tt := range tests {
		got := n.DirectionTo(tt.tx, tt.ty)
		if got != tt.dir {
			t.Errorf("DirectionTo(%d,%d): expected dir=%d, got %d", tt.tx, tt.ty, tt.dir, got)
		}
	}
}

func TestIsShopNPC(t *testing.T) {
	if !IsShopNPC(15) {
		t.Error("Type 15 (ShopKeeper) should be a shop NPC")
	}
	if !IsShopNPC(20) {
		t.Error("Type 20 (Howard) should be a shop NPC")
	}
	if !IsShopNPC(24) {
		t.Error("Type 24 (Tom) should be a shop NPC")
	}
	if !IsShopNPC(90) {
		t.Error("Type 90 (Gail) should be a shop NPC")
	}
	if IsShopNPC(10) {
		t.Error("Type 10 (Slime) should not be a shop NPC")
	}
	if IsShopNPC(31) {
		t.Error("Type 31 (Demon) should not be a shop NPC")
	}
}

func TestShopNPCProperties(t *testing.T) {
	for _, id := range []int{15, 19, 20, 24, 25, 26} {
		npcType := NpcTypes[id]
		if npcType == nil {
			t.Errorf("Shop NPC type %d not found", id)
			continue
		}
		if npcType.AggroRange != 0 {
			t.Errorf("Shop NPC %d should have AggroRange=0", id)
		}
		if npcType.WanderRange != 0 {
			t.Errorf("Shop NPC %d should have WanderRange=0", id)
		}
	}
}

func TestNPCToNpcAppear(t *testing.T) {
	n := NewNPC(42, NpcTypes[10], "default", 30, 40)
	msg := n.ToNpcAppear()

	if msg.ObjectId != 42 {
		t.Errorf("Expected ObjectId=42, got %d", msg.ObjectId)
	}
	if msg.Name != "Slime" {
		t.Errorf("Expected name=Slime, got %s", msg.Name)
	}
	if msg.Position.X != 30 || msg.Position.Y != 40 {
		t.Errorf("Expected position (30,40), got (%d,%d)", msg.Position.X, msg.Position.Y)
	}
}

func TestNPCToEntityInfo(t *testing.T) {
	n := NewNPC(42, NpcTypes[11], "default", 30, 40)
	info := n.ToEntityInfo()

	if info.EntityType != 2 {
		t.Errorf("Expected EntityType=2, got %d", info.EntityType)
	}
	if info.Name != "Skeleton" {
		t.Errorf("Expected name=Skeleton, got %s", info.Name)
	}
}

func TestNPCNamesHaveNoTownOrAbbreviation(t *testing.T) {
	// Town variants share a plain name; the Side field tells them apart
	if a, e := NpcTypes[1013], NpcTypes[1014]; a.Name != "Guard" || e.Name != "Guard" || a.Side == e.Side {
		t.Errorf("guards should both be called Guard, with different sides: %q/%d, %q/%d", a.Name, a.Side, e.Name, e.Side)
	}
	for _, nt := range NpcTypes {
		if strings.HasSuffix(nt.Name, "-Elvine") || strings.HasSuffix(nt.Name, "-Aresden") {
			t.Errorf("NPC type %d still shows its town: %s", nt.ID, nt.Name)
		}
	}
}

func TestNPCStatusBitsCarryTheSide(t *testing.T) {
	status := func(typeID int) int32 { return NewNPC(1, NpcTypes[typeID], "m", 1, 1).StatusBits() }
	if status(10) != NpcStatusHostile {
		t.Error("a Slime should be hostile to everyone")
	}
	if status(1013) != NpcStatusCitizen|NpcStatusAresden || status(1014) != NpcStatusCitizen {
		t.Error("guards should carry their town")
	}
	if status(25) != 0 {
		t.Error("William is neutral")
	}
	summon := NewNPC(1, NpcTypes[10], "m", 1, 1)
	summon.MasterID, summon.MasterSide = 7, SideElvine
	if summon.StatusBits() != NpcStatusCitizen {
		t.Error("an Elvine citizen's summon should show as Elvine")
	}
}

func TestRespawnClearsSpells(t *testing.T) {
	n := NewNPC(1, NpcTypes[10], "test", 5, 5)
	n.PolymorphUntil = time.Now().Add(time.Minute)
	n.HeldUntil = time.Now().Add(time.Minute)
	n.Respawn()
	if n.IsPolymorphed(time.Now()) || !n.HeldUntil.IsZero() {
		t.Error("a respawned NPC should not still be polymorphed or held")
	}
}
