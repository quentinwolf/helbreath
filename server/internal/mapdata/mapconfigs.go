package mapdata

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// MapConfigDir is the folder (inside the map edits folder) holding one
// <map>.txt per map. See mapconfig.go for the format.
const MapConfigDir = "maps"

//go:embed maps/*.txt
var builtinMapConfigs embed.FS

// MapConfigs are every map's config, by lower-case map name.
type MapConfigs map[string]*MapConfig

// Get is a map's config (name ignoring case), or nil.
func (cs MapConfigs) Get(name string) *MapConfig {
	return cs[strings.ToLower(name)]
}

// Names are the maps' names as spelled by their files, sorted.
func (cs MapConfigs) Names() []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	sort.Strings(out)
	return out
}

var (
	// configChecker checks what mapdata can't (monster and NPC names: the game
	// package sets it). nil = no extra checks.
	configChecker  func(*MapConfig) []error
	currentConfigs atomic.Pointer[MapConfigs]
	builtinOnce    sync.Once
	builtinConfigs MapConfigs
)

// SetConfigChecker adds a check every config file must pass to be loaded.
// Call it from an init function, before any config is read.
func SetConfigChecker(f func(*MapConfig) []error) { configChecker = f }

// Configs are the map configs in use: the last SetConfigs, or the built-in
// copies until then (tools and tests).
func Configs() MapConfigs {
	if cs := currentConfigs.Load(); cs != nil {
		return *cs
	}
	return BuiltinConfigs()
}

// SetConfigs makes cs the map configs in use (the server, after loading or reloading).
func SetConfigs(cs MapConfigs) {
	currentConfigs.Store(&cs)
}

// BuiltinConfigs are the copies built into the binary. A built-in file with an
// error is left out (a test checks they all load).
func BuiltinConfigs() MapConfigs {
	builtinOnce.Do(func() {
		builtinConfigs, _ = readMapConfigs(builtinMapConfigs, "maps", nil)
	})
	return builtinConfigs
}

// LoadMapConfigs reads dir/maps/*.txt. A file with an error keeps its
// built-in copy (or, with prev, the copy loaded before), and the errors are
// returned. dir "" or a folder without maps/ gives the built-in copies.
func LoadMapConfigs(dir string, prev MapConfigs) (MapConfigs, []error) {
	if dir == "" {
		return BuiltinConfigs(), nil
	}
	folder := filepath.Join(dir, MapConfigDir)
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return BuiltinConfigs(), nil
	}
	fallback := BuiltinConfigs()
	if prev != nil {
		fallback = prev
	}
	return readMapConfigs(os.DirFS(folder), ".", fallback)
}

func readMapConfigs(fsys fs.FS, root string, fallback MapConfigs) (MapConfigs, []error) {
	out := MapConfigs{}
	var errs []error
	entries, err := fs.ReadDir(fsys, root)
	if err != nil {
		return fallback, []error{err}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".txt") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		lower := strings.ToLower(name)
		if other, dup := out[lower]; dup {
			errs = append(errs, fmt.Errorf("%s.txt and %s.txt are the same map", other.Name, name))
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(root, e.Name())))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		c, perrs := ParseMapConfig(name, data)
		if len(perrs) == 0 {
			perrs = CheckConfig(c)
		}
		if len(perrs) > 0 {
			errs = append(errs, perrs...)
			if old := fallback.Get(name); old != nil {
				out[lower] = old
			}
			continue
		}
		out[lower] = c
	}
	return out, errs
}

// CheckConfig is every check a config must pass to be loaded (after parsing).
func CheckConfig(c *MapConfig) []error {
	if errs := c.check(); len(errs) > 0 {
		return errs
	}
	if configChecker != nil {
		return configChecker(c)
	}
	return nil
}

// check finds problems that span lines.
func (c *MapConfig) check() []error {
	var errs []error
	for _, n := range c.NPCs {
		if _, err := c.NpcPosition(n); err != nil {
			errs = append(errs, fmt.Errorf("%s.txt line %d: %w", c.Name, n.Line, err))
		}
	}
	for _, s := range c.SpotMobs {
		if s.Area.X2 < s.Area.X1 || s.Area.Y2 < s.Area.Y1 {
			errs = append(errs, fmt.Errorf("%s.txt line %d: spot-mob-generator %d: right/bottom must not be less than left/top", c.Name, s.Line, s.Num))
		}
		if s.Max < 0 || s.Max > 500 {
			errs = append(errs, fmt.Errorf("%s.txt line %d: spot-mob-generator %d: max %d (must be 0-500)", c.Name, s.Line, s.Num, s.Max))
		}
	}
	return errs
}

// ConfigStamp identifies the version of dir/maps/*.txt (names, times, sizes),
// so a change can be noticed without reading the files. "" without the folder.
func ConfigStamp(dir string) string {
	if dir == "" {
		return ""
	}
	entries, err := os.ReadDir(filepath.Join(dir, MapConfigDir))
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !e.IsDir() {
			fmt.Fprintf(&b, "%s:%d:%d;", e.Name(), info.ModTime().UnixNano(), info.Size())
		}
	}
	return b.String()
}
