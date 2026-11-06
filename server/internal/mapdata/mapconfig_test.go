package mapdata

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleConfig = `// test map
[MAP-INFO]
map-location  = aresden
initial-point = 1  140 49
maximum-object		= 600
random-mob-generator	= 1  1
fixed-dayornight-mode = 1

//-Teleportation-Set----SX-----SY-----DestMapName---------DX-----DY----Dir---;
teleport-loc     =     54     33     aresden             222    136    4
waypoint          =     1      66    38
npc               =   Howard               1         1 2 3 4 5 6 7 8 9 10         1
no-attack-area           = 1       27 20 35 28
spot-mob-generator	= 1  1    106 43 123 55      Slime  20   // north pit
fish-point = 0   35 241
mineral-point = 3  127 47
[END-MAP-INFO]
`

func TestParseMapConfig(t *testing.T) {
	c, errs := ParseMapConfig("test", []byte(strings.ReplaceAll(sampleConfig, "\n", "\r\n")))
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if c.Location != "aresden" || c.MaxObjects != 600 || !c.FixedDay || !c.RandomMobs.On || c.RandomMobs.Level != 1 {
		t.Errorf("settings: %+v", c)
	}
	if len(c.InitialPoints) != 1 || c.InitialPoints[0] != (IndexedPoint{1, 140, 49}) {
		t.Errorf("initial point: %v", c.InitialPoints)
	}
	if len(c.Teleports) != 1 || c.Teleports[0] != (TeleportLoc{54, 33, "aresden", 222, 136, 4, 10, ""}) {
		t.Errorf("teleports: %v", c.Teleports)
	}
	if len(c.SpotMobs) != 1 || c.SpotMobs[0].Monster != "Slime" || c.SpotMobs[0].Max != 20 || c.SpotMobs[0].Area != (Rect{106, 43, 123, 55}) {
		t.Errorf("spot mobs: %+v", c.SpotMobs)
	}
	if len(c.NPCs) != 1 || c.NPCs[0].Name != "Howard" || len(c.NPCs[0].Waypoints) != 10 {
		t.Fatalf("npcs: %+v", c.NPCs)
	}
	if at, err := c.NpcPosition(c.NPCs[0]); err != nil || at != (Point{66, 38}) {
		t.Errorf("Howard should stand on waypoint 1 (66,38), got %v %v", at, err)
	}
	if len(c.NoAttack) != 1 || !c.NoAttack[0].Contains(30, 25) || c.NoAttack[0].Contains(36, 25) {
		t.Errorf("no-attack: %v", c.NoAttack)
	}
	if len(c.FishPoints) != 1 || len(c.MineralPts) != 1 {
		t.Errorf("fish %v minerals %v", c.FishPoints, c.MineralPts)
	}
	if got := len(c.Lines); got != strings.Count(sampleConfig, "\n") {
		t.Errorf("every line should be kept, got %d", got)
	}
	if l := c.Lines[13]; l.Key != "spot-mob-generator" || l.Comment != "north pit" {
		t.Errorf("line 14: %+v", l)
	}
}

func TestParseMapConfigErrors(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"spot-mob-generator = 1 1 10 10 20 20 10 5", "monster's name"},
		{"teleprot-loc = 1 2 aresden 3 4 5", "unknown setting"},
		{"teleport-loc = 1 2 aresden x 4 5", "isn't a number"},
		{"waypoint = 1 2\n", "needs 3 numbers"},
		{"waypoint = 1 2 3\nwaypoint = 1 4 5", "defined twice"},
	} {
		_, errs := ParseMapConfig("bad", []byte(tc.line))
		if len(errs) == 0 || !strings.Contains(errs[0].Error(), tc.want) {
			t.Errorf("%q: want an error about %q, got %v", tc.line, tc.want, errs)
		}
		if len(errs) > 0 && !strings.Contains(errs[0].Error(), "bad.txt line ") {
			t.Errorf("errors should say the file and line: %v", errs[0])
		}
	}
	c, _ := ParseMapConfig("bad", []byte("npc = Howard 1 7 7 7 7 7 7 7 7 7 7 1"))
	if errs := c.check(); len(errs) == 0 {
		t.Error("an npc on an undefined waypoint should be an error")
	}
}

func TestBuiltinMapConfigsLoad(t *testing.T) {
	cs, errs := readMapConfigs(builtinMapConfigs, "maps", nil)
	for _, err := range errs {
		t.Error(err)
	}
	if len(cs) < 80 {
		t.Errorf("expected a config for each of the port's 82 maps, got %d", len(cs))
	}
	for _, m := range []string{"aresden", "elvine", "middleland", "Cmdhall_1", "default"} {
		if cs.Get(m) == nil {
			t.Errorf("no config for %s", m)
		}
	}
	if c := cs.Get("cmdhall_1"); c != nil && c.Name != "Cmdhall_1" {
		t.Errorf("names should be spelled as the files: %q", c.Name)
	}
}

func TestLoadMapConfigsKeepsTheOldCopyOfABrokenFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, MapConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(dir, MapConfigDir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("aresden.txt", "maximum-object = 10\n")
	write("elvine.txt", "maximum-object = 20\n")
	cs, errs := LoadMapConfigs(dir, nil)
	if len(errs) > 0 || cs.Get("aresden").MaxObjects != 10 || len(cs) != 2 {
		t.Fatalf("first load: %v %v", cs, errs)
	}
	write("aresden.txt", "maximum-object = ten\n")
	cs2, errs := LoadMapConfigs(dir, cs)
	if len(errs) != 1 {
		t.Fatalf("want one error, got %v", errs)
	}
	if cs2.Get("aresden").MaxObjects != 10 {
		t.Error("a broken file should keep the copy loaded before")
	}
	if stamp := ConfigStamp(dir); stamp == "" || !strings.Contains(stamp, "aresden.txt") {
		t.Errorf("stamp %q", stamp)
	}
}
