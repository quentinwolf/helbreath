package npc

import (
	"strings"
	"testing"
)

func TestTypeForConfigName(t *testing.T) {
	for name, want := range map[string]int{
		"Slime": 10, "slime": 10, "Guard-Aresden": 1013, "Guard-Neutral": 1015,
		"Orge": 29, "Ogre": 29, "Dark-Elf": 1059, "Gandlf": 19, "ShopKeeper-W": 15,
	} {
		if got, err := TypeForConfigName(name); err != nil || got != want {
			t.Errorf("%s: got %d %v, want %d", name, got, err, want)
		}
	}
	if _, err := TypeForConfigName("Guard"); err == nil || !strings.Contains(err.Error(), "Guard-Aresden") {
		t.Errorf("a shared name should be refused with a hint, got %v", err)
	}
	if _, err := TypeForConfigName("Barlog"); err == nil {
		t.Error("an unknown name should be refused")
	}
}

func TestConfigNameRoundTrips(t *testing.T) {
	for _, id := range []int{10, 29, 1013, 1014, 1059, 15} {
		name := ConfigName(id)
		if got, err := TypeForConfigName(name); err != nil || got != id {
			t.Errorf("type %d: name %q reads back as %d %v", id, name, got, err)
		}
	}
	if ConfigName(1059) != "Dark-Elf" || ConfigName(1013) != "Guard-Aresden" {
		t.Errorf("got %q, %q", ConfigName(1059), ConfigName(1013))
	}
}
