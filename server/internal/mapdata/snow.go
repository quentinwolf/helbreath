package mapdata

import "strings"

// IsSnowMap reports whether it always snows on a map, whatever the month: the
// original server turned snow on for any map whose name starts with "icebound".
func IsSnowMap(name string) bool {
	return strings.HasPrefix(strings.ToLower(name), "icebound")
}
