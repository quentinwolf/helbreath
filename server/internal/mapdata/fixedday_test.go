package mapdata

import "testing"

func TestIsFixedDayMap(t *testing.T) {
	for _, name := range []string{"Toh1", "toh2", "TOH3", "dglv4", "Cmdhall_1", "wrhus_2", "middled1n"} {
		if !IsFixedDayMap(name) {
			t.Errorf("%s should be a fixed-day (no weather) map", name)
		}
	}
	for _, name := range []string{"aresden", "elvine", "middleland", "huntzone1", "icebound", "2ndmiddle"} {
		if IsFixedDayMap(name) {
			t.Errorf("%s should have weather", name)
		}
	}
}
