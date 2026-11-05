package db

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileStoreSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "save.json")

	s, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	accID, err := s.CreateAccount(ctx, "zac", "hash", "")
	if err != nil {
		t.Fatal(err)
	}
	charID, err := s.CreateCharacter(ctx, accID, "Mage", 0, 0, 0, 0, 0, 10, 10, 10, 20, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SaveCharacter(ctx, charID, "elvine", 40, 50, 3, 25, 12345, 80, 60, 40,
		10, 10, 10, 20, 10, 10, 2, 2, 999, 0, 1, 100, "[]", "[]", "[10]"); err != nil {
		t.Fatal(err)
	}

	// "Restart": load a fresh store from the same file
	s2, err := NewFileStore(path)
	if err != nil {
		t.Fatal(err)
	}
	id, hash, _, err := s2.GetAccountByUsername(ctx, "zac")
	if err != nil || id != accID || hash != "hash" {
		t.Fatalf("account not restored: id=%d hash=%q err=%v", id, hash, err)
	}
	c, err := s2.GetCharacterByID(ctx, charID, accID)
	if err != nil {
		t.Fatal(err)
	}
	if c.Level != 25 || c.MapName != "elvine" || c.Gold != 999 || c.SpellDataJSON != "[10]" {
		t.Errorf("character not restored: %+v", c)
	}

	// New records keep counting up instead of reusing IDs
	acc2, _ := s2.CreateAccount(ctx, "friend", "hash2", "")
	if acc2 == accID {
		t.Error("account ID reused after restart")
	}
}

func TestFileStoreRemembersGodMode(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "save.json")

	s, _ := NewFileStore(path)
	accID, _ := s.CreateAccount(ctx, "gm", "hash", "")
	charID, _ := s.CreateCharacter(ctx, accID, "GM", 0, 0, 0, 0, 0, 10, 10, 10, 20, 10, 10)
	if err := s.SaveGodMode(ctx, charID, true); err != nil {
		t.Fatal(err)
	}

	s2, _ := NewFileStore(path)
	c, err := s2.GetCharacterByID(ctx, charID, accID)
	if err != nil || !c.GodMode {
		t.Errorf("god mode should survive a restart: %+v err=%v", c, err)
	}
}

func TestFileStoreStartsEmptyWithoutFile(t *testing.T) {
	s, err := NewFileStore(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := s.CountCharacters(context.Background(), 1); n != 0 {
		t.Errorf("expected empty store, got %d characters", n)
	}
}
