package mapdata

import "strings"

// fixedDayMaps are the maps with "fixed-dayornight-mode = 1" in the original
// server's MAPDATA/*.txt (HB Server Files 3.51): indoor maps and dungeons.
// The original server never gave them weather and always told the client it
// was daytime there (Game.cpp: weather is 0 and day/night is 1 for these maps).
// Names are lower case; the last group are maps only this port has, assumed
// indoor like their 3.51 counterparts.
var fixedDayMaps = map[string]bool{
	// Town buildings
	"bsmith_1": true, "bsmith_1f": true, "bsmith_2": true, "bsmith_2f": true,
	"cath_1": true, "cath_2": true,
	"cityhall_1": true, "cityhall_2": true,
	"cmdhall_1": true, "cmdhall_2": true,
	"gldhall_1": true, "gldhall_2": true,
	"gshop_1": true, "gshop_1f": true, "gshop_2": true, "gshop_2f": true,
	"wrhus_1": true, "wrhus_1f": true, "wrhus_2": true, "wrhus_2f": true,
	"arewrhus": true, "elvwrhus": true,
	"wzdtwr_1": true, "wzdtwr_2": true,
	"resurr1": true, "resurr2": true,
	"arejail": true, "elvjail": true,
	"arebrk11": true, "arebrk12": true, "arebrk21": true, "arebrk22": true,
	"elvbrk11": true, "elvbrk12": true, "elvbrk21": true, "elvbrk22": true,
	// Dungeons and other indoor areas
	"aresdend1": true, "elvined1": true,
	"dglv2": true, "dglv3": true, "dglv4": true,
	"middled1n": true, "middled1x": true,
	"toh1": true, "toh2": true, "toh3": true,
	"abaddon": true, "druncncity": true, "maze": true,
	"inferniaa": true, "inferniab": true,
	"godh": true,
	"fightzone4": true, "fightzone5": true, "fightzone6": true, "fightzone7": true, "fightzone8": true,
	// Port-only maps
	"whouse": true, "whouse2": true,
	"abarracks": true, "ebarracks": true, "mbarracks": true,
	"market": true, "towerofh": true,
}

// IsFixedDayMap reports whether a map never has weather and is always daytime.
func IsFixedDayMap(name string) bool {
	return fixedDayMaps[strings.ToLower(name)]
}
