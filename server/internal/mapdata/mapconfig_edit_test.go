package mapdata

import (
	"strings"
	"testing"
)

const editSample = `// shop
[MAP-INFO]
map-location  = aresden

// Doors
teleport-loc  =  54 33  aresden  222 136 4
teleport-loc  =  55 33  aresden  222 136 4
teleport-loc  =  60 40  arefarm  -1 -1 5

//-WayPoint-Set
waypoint = 1 66 38
waypoint = 2 66 38
waypoint = 5 70 40
waypoint = 6 70 40
npc = Howard 1 1 2 1 2 1 2 1 2 1 2 1
npc = Tom    2 5 6 5 6 5 6 5 6 5 6 1
npc = Tom    2 5 6 5 6 5 6 5 6 5 6 1

// Slimes
spot-mob-generator = 1 1  10 10 20 20  Slime 5   // north pit
spot-mob-generator = 2 1  30 30 40 40  Orc 3
//spot-mob-generator = 7 1  30 30 40 40  51 3   // commented out
[END-MAP-INFO]
`

func parseOK(t *testing.T, text string) *MapConfig {
	t.Helper()
	c, errs := ParseMapConfig("shop", []byte(text))
	if len(errs) > 0 {
		t.Fatalf("result doesn't parse: %v\n%s", errs, text)
	}
	if errs := c.check(); len(errs) > 0 {
		t.Fatalf("result fails checks: %v\n%s", errs, text)
	}
	return c
}

func TestEditDoors(t *testing.T) {
	doors := []DoorTile{
		{X: 54, Y: 33, DestMap: "aresden", DestX: 222, DestY: 136, Dir: 4},   // unchanged
		{X: 55, Y: 33, DestMap: "aresden", DestX: 230, DestY: 140, Dir: 4},   // new arrival
		{X: 56, Y: 33, DestMap: "aresden", DestX: 222, DestY: 136, Dir: 4},   // new tile
		{X: 10, Y: 10, DestMap: "middleland", DestX: 1, DestY: 2, Note: "x"}, // new door
	}
	exists := func(m string) bool { return !strings.EqualFold(m, "arefarm") }
	out, err := EditMapConfig("shop", []byte(editSample), MapEdit{Doors: &doors, MapExists: exists})
	if err != nil {
		t.Fatal(err)
	}
	c := parseOK(t, out)
	if !c.ExactDoors {
		t.Error("saving doors should set exact-doors")
	}
	got := map[[2]int]TeleportLoc{}
	for _, d := range c.Teleports {
		got[[2]int{d.X, d.Y}] = d
	}
	if len(got) != 5 || got[[2]int{55, 33}].DestX != 230 || got[[2]int{56, 33}].DestMap != "aresden" || got[[2]int{10, 10}].Comment != "x" {
		t.Errorf("doors: %+v\n%s", got, out)
	}
	if got[[2]int{60, 40}].DestMap != "arefarm" {
		t.Error("a door to a map the port lacks should be left alone")
	}
	if !strings.Contains(out, "teleport-loc  =  54 33  aresden  222 136 4") {
		t.Error("an unchanged line should keep its exact text")
	}
	// the new aresden tile goes with the other aresden doors
	if strings.Index(out, "56     33") > strings.Index(out, "arefarm") {
		t.Errorf("new tile should follow its door's lines:\n%s", out)
	}
	// Saving the same doors again changes nothing
	again, _ := EditMapConfig("shop", []byte(out), MapEdit{Doors: &doors, MapExists: exists})
	if again != out {
		t.Errorf("a second save should be a no-op:\n%s\n---\n%s", out, again)
	}
}

func TestEditSpawns(t *testing.T) {
	c0 := parseOK(t, editSample)
	spawns := []SpawnEdit{
		{Src: "spot:1", Name: "Slime", Spot: true, Area: Rect{10, 10, 20, 20}, Max: 5, Note: "north pit"}, // unchanged
		{Src: "spot:2", Name: "Orc", Spot: true, Area: Rect{31, 30, 41, 40}, Max: 4},                      // moved
		{Name: "Zombie", Spot: true, Area: Rect{1, 1, 5, 5}, Max: 2, SpawnOptions: SpawnOptions{Respawn: 60}},
		{Src: "npc:" + itoa(c0.NPCs[0].Line), Name: "Howard", X: 67, Y: 39, SpawnOptions: SpawnOptions{Dir: 4}},
		{Src: "npc:" + itoa(c0.NPCs[1].Line), Name: "Tom", X: 70, Y: 40},
		// the second Tom is gone; a new Gandlf
		{Name: "Gandlf", X: 50, Y: 33},
	}
	out, err := EditMapConfig("shop", []byte(editSample), MapEdit{Spawns: &spawns})
	if err != nil {
		t.Fatal(err)
	}
	c := parseOK(t, out)
	if len(c.SpotMobs) != 3 {
		t.Fatalf("spots: %+v\n%s", c.SpotMobs, out)
	}
	if !strings.Contains(out, "spot-mob-generator = 1 1  10 10 20 20  Slime 5   // north pit") {
		t.Error("the unchanged spot line should be untouched")
	}
	for _, s := range c.SpotMobs {
		switch s.Monster {
		case "Orc":
			if s.Num != 2 || s.Area != (Rect{31, 30, 41, 40}) || s.Max != 4 {
				t.Errorf("orc spot: %+v", s)
			}
		case "Zombie":
			if s.Num != 8 || s.Respawn != 60 {
				t.Errorf("new spot should be number 8 (after the commented-out 7) with respawn=60: %+v", s)
			}
		}
	}
	if len(c.NPCs) != 3 {
		t.Fatalf("npcs: %+v\n%s", c.NPCs, out)
	}
	pos := map[string]Point{}
	for _, n := range c.NPCs {
		p, _ := c.NpcPosition(n)
		pos[n.Name] = p
		if n.Name == "Howard" && n.Dir != 4 {
			t.Errorf("Howard should face 4: %+v", n)
		}
	}
	if pos["Howard"] != (Point{67, 39}) || pos["Tom"] != (Point{70, 40}) || pos["Gandlf"] != (Point{50, 33}) {
		t.Errorf("positions %v\n%s", pos, out)
	}
	// Howard owned waypoints 1 and 2: they moved, no new waypoint
	if c.Waypoints[1] != (Point{67, 39}) || c.Waypoints[2] != (Point{67, 39}) {
		t.Errorf("Howard's own waypoints should move with him: %v", c.Waypoints)
	}
	if strings.Count(out, "// Slimes") != 1 || !strings.Contains(out, "//-WayPoint-Set") {
		t.Error("comments should stay")
	}
	again, _ := EditMapConfig("shop", []byte(out), MapEdit{Spawns: &spawns})
	if again != out {
		t.Errorf("a second save should be a no-op:\n%s\n---\n%s", out, again)
	}
}

func TestEditSharedWaypointsGetANewOne(t *testing.T) {
	text := "waypoint = 1 10 10\nnpc = Tom 1 1 1 1 1 1 1 1 1 1 1 1\nnpc = Tom 1 1 1 1 1 1 1 1 1 1 1 1\n"
	c0 := parseOK(t, text)
	spawns := []SpawnEdit{
		{Src: "npc:" + itoa(c0.NPCs[0].Line), Name: "Tom", X: 10, Y: 10},
		{Src: "npc:" + itoa(c0.NPCs[1].Line), Name: "Tom", X: 20, Y: 25},
	}
	out, err := EditMapConfig("shop", []byte(text), MapEdit{Spawns: &spawns})
	if err != nil {
		t.Fatal(err)
	}
	c := parseOK(t, out)
	if c.Waypoints[1] != (Point{10, 10}) || c.Waypoints[2] != (Point{20, 25}) {
		t.Errorf("the moved Tom should get waypoint 2, the other keeps 1: %v\n%s", c.Waypoints, out)
	}
}

func TestEditCRLFAndSettingAdded(t *testing.T) {
	text := "[MAP-INFO]\r\nmap-location = x\r\n[END-MAP-INFO]\r\n"
	doors := []DoorTile{{X: 1, Y: 2, DestMap: "y", DestX: 3, DestY: 4}}
	out, err := EditMapConfig("x", []byte(text), MapEdit{Doors: &doors})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(out, "\r\n") != strings.Count(out, "\n") {
		t.Error("CRLF files should stay CRLF")
	}
	c := parseOK(t, out)
	if !c.ExactDoors || len(c.Teleports) != 1 {
		t.Errorf("%+v\n%s", c, out)
	}
	if strings.Index(out, "teleport-loc") > strings.Index(out, "[END-MAP-INFO]") {
		t.Error("new lines go before [END-MAP-INFO]")
	}
}

func TestEditMapConfigSettings(t *testing.T) {
	src := "[MAP-INFO]\nmap-location = elvine\nmaximum-object\t\t= 600   // cap\nrandom-mob-generator\t= 1  1\n[END-MAP-INFO]\n"
	out, err := EditMapConfig("x", []byte(src), MapEdit{Settings: map[string]string{
		"maximum-object": "200", "random-mob-max": "40", "random-mob-groups": "1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "maximum-object\t\t= 200   // cap\n") {
		t.Errorf("value should change in place, keeping spacing and comment:\n%s", out)
	}
	c, errs := ParseMapConfig("x", []byte(out))
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	if c.MaxObjects != 200 || !c.RandomMobs.MaxSet || c.RandomMobs.Max != 40 || !c.RandomMobs.Groups || !c.RandomMobs.On {
		t.Errorf("settings not read back: %+v %d", c.RandomMobs, c.MaxObjects)
	}
	// Removing them again gives back the original file
	back, err := EditMapConfig("x", []byte(out), MapEdit{Settings: map[string]string{
		"maximum-object": "600", "random-mob-max": "", "random-mob-groups": "",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if back != src {
		t.Errorf("round trip changed the file:\n%q\nwant\n%q", back, src)
	}
	if _, err := EditMapConfig("x", []byte(src), MapEdit{Settings: map[string]string{"pk-mode": "1"}}); err == nil {
		t.Error("only editable settings can be set")
	}
}
