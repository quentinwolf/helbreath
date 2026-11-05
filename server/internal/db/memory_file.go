package db

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
)

// memSnapshot is the on-disk form of a MemoryStore.
type memSnapshot struct {
	Version       int            `json:"version"`
	NextAccountID int            `json:"nextAccountId"`
	NextCharID    int            `json:"nextCharId"`
	Accounts      []memAccount   `json:"accounts"`
	Characters    []CharacterRow `json:"characters"`
}

// NewFileStore is a MemoryStore that persists to a JSON file: loaded at start
// (if it exists) and rewritten after every change. Handy for local play
// without PostgreSQL; not meant for many concurrent players.
func NewFileStore(path string) (*MemoryStore, error) {
	m := NewMemoryStore()
	m.path = path

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	var snap memSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for i := range snap.Accounts {
		acc := snap.Accounts[i]
		m.accounts[acc.ID] = &acc
		m.accountsByName[acc.Username] = acc.ID
	}
	for i := range snap.Characters {
		c := snap.Characters[i]
		m.characters[c.ID] = &c
	}
	m.nextAccountID = max(snap.NextAccountID, 1)
	m.nextCharID = max(snap.NextCharID, 1)
	return m, nil
}

// saveLocked writes the store to its file, if it has one. Callers hold m.mu.
// It writes a temp file and renames it over the old one, so a crash mid-write
// never leaves a half-written save.
func (m *MemoryStore) saveLocked() {
	if m.path == "" {
		return
	}
	snap := memSnapshot{Version: 1, NextAccountID: m.nextAccountID, NextCharID: m.nextCharID}
	for _, acc := range m.accounts {
		snap.Accounts = append(snap.Accounts, *acc)
	}
	for _, c := range m.characters {
		snap.Characters = append(snap.Characters, *c)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		log.Printf("Save file: encode failed: %v", err)
		return
	}
	if dir := filepath.Dir(m.path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Printf("Save file: %v", err)
			return
		}
	}
	tmp := m.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("Save file: write failed: %v", err)
		return
	}
	if err := os.Rename(tmp, m.path); err != nil {
		log.Printf("Save file: replace failed: %v", err)
	}
}
