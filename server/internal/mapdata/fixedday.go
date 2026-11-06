package mapdata

// IsFixedDayMap reports whether a map never has weather and is always daytime:
// "fixed-dayornight-mode = 1" in its config (indoor maps and dungeons). The
// original server never gave those maps weather and always told the client it
// was daytime there.
func IsFixedDayMap(name string) bool {
	c := Configs().Get(name)
	return c != nil && c.FixedDay
}
