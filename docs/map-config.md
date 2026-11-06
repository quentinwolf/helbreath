# Map config files

Every map has one plain-text config, `server/internal/mapdata/maps/<map>.txt`, named exactly
like its `.amd` file (`Cmdhall_1.txt` for `Cmdhall_1.amd`). It holds the map's doors, spawns,
town NPCs and rules, in the original server's `MAPDATA/*.txt` syntax, so lines from the
original files can be pasted in as they are.

The running server reloads the folder within a second of a save (`-mapedits`, default
`server/internal/mapdata`). Changed maps get their doors rebuilt and their NPCs respawned;
admins in game get a notice. A file with an error isn't loaded: the server logs the file,
line and problem, tells the admins, and keeps that map's previous version. The files are
also built into the server binary as the fallback.

The map editor ([map-editor.md](map-editor.md)) saves doors and spawns into these files,
rewriting only the lines that change: comments, blank lines and settings it doesn't edit stay
as they are. `walkable.json` (tile walkability patches over the `.amd`) is the one editor file
that isn't part of the map configs.

## Syntax

```
// a comment; "//" can also end a line
[MAP-INFO]                                  // section markers are allowed and ignored
key = value value value                     // values separated by spaces or tabs
spot-mob-generator = 1  1  106 43 123 55  Slime  20
[END-MAP-INFO]
```

- One setting per line. The `=` is optional (`key value value` works too). Keys ignore case.
- Unknown keys are errors (they're nearly always typos).
- Monsters and NPCs are written by **name**: our NPC name (`Slime`, `Ogre`, `Gandlf`,
  `ShopKeeper-W`) or the original's name for types we show under a shared name
  (`Guard-Aresden`, `Guard-Elvine`, `Guard-Neutral`). Names any case. A name several types
  share (`Guard`, `Soldier`) is refused with a hint. The original's spot-mob-generator used
  mob codes from a table inside its server instead; `mapimport` converts them.
- Tiles are map tile coordinates; rectangles are `left top right bottom`, corners included.
- Port options at the end of a `spot-mob-generator` or `npc` line: `dir=N` (facing on spawn, 1 =
  north clockwise to 8) and `respawn=N` (seconds; default the NPC type's). Example:
  `npc = ShopKeeper-W 2 1 2 3 4 5 6 7 8 9 10 1 dir=4`.

## Settings

**Used** = the server reads it. **Kept** = parsed or kept in the file for later; no effect yet
(see [STATUS.md](STATUS.md)).

| Setting | Values | State |
|---|---|---|
| `map-location` | town side (`aresden`, `elvine`) or the map's own name | Kept |
| `initial-point` | num x y: where you arrive on the map / respawn | Kept (doors to `-1 -1` still use the map centre) |
| `teleport-loc` | x y dest-map dest-x dest-y dir: a door tile. dest `-1 -1` = the dest map's arrival point | **Used** |
| `exact-doors` | 1: doors work on exactly the tiles listed (port setting, set when the editor saves a map's doors). Without it a tile needs the `.amd` teleport flag and borrows a door entry up to 2 tiles away | **Used** |
| `waypoint` | num x y | **Used** (positions for `npc` lines) |
| `npc` | name move-type wp1 … wp10 prefix: a town NPC on its waypoints (move type 1 in order, 2 random) | **Used** (stands on wp1; walking between waypoints not ported) |
| `spot-mob-generator` | num type left top right bottom monster max | **Used** for type 1: up to *max* alive, spawning and wandering in the rectangle. Type 2 (waypoint routes) not ported |
| `maximum-object` | n: monster cap for the map | **Used**: when the spot rectangles add up to more, each gets its share; the random mob generator fills up to n - 30 |
| `fixed-dayornight-mode` | 1: always day, never weather (indoors, dungeons) | **Used** |
| `random-mob-generator` | on level: random monsters anywhere by the original's level table (1 = towns: Slime, Giant Ant, Rabbit, Cat, Orc) up to `maximum-object` - 30 | **Used** |
| `npc-avoidrect` | num rect: where random monsters don't spawn | **Used** |
| `no-attack-area` | num rect. `-10 -10 -10 -10` = **the whole map** (town buildings); other rectangles count only with a top above 0 | **Used**: nobody inside can cast attack spells; a player inside can't be hurt by melee or spells (players or monsters), and monsters don't go for them |
| `level-limit`, `upper-level-limit` | lowest / highest level allowed in | Kept |
| `pk-mode`, `CitizenLimit`, `RecallImpossible`, `limited-cast`, `drop-type`, `type`, `not-permanent-map` | map rules | Kept |
| `fish-point`, `max-fish` | num x y; n | Kept |
| `mineral-generator`, `mineral-point`, `max-mineral` | on level; num x y; n | Kept |
| `strike-point`, `strategic-point`, `energy-sphere-creation-point`, `energy-sphere-goal-point`, `energy-sphere-auto-creation` | Crusade / Energy Sphere | Kept |
| `item-event`, `mobevent-amount`, `boss-rate`, `extra-drop` | events, drops | Kept |
| `Heldenian*`, `Apocalypse*`, `DynamicGate*` | Heldenian and Apocalypse events | Kept |

## Where the files came from

- 55 maps: the original server's files (HB Server Files 3.51, `HelbreathServer-main/Files/GameServers/*/MAPDATA`),
  converted by `server/tools/mapimport` (mob codes to names; lines the original server never
  used are commented out with the reason, e.g. code 51 is ambiguous, Cat and Troll were
  disabled in HGServer). Each file's header says which original it came from.
- 27 maps only this port has (event maps, barracks, `whouse`...): their doors and fixed-day flag,
  carried over from the port's old hand-typed tables.
- Port additions are marked with a comment, for example:
  - stand-in exits on `default` and `2ndmiddle`, whose original exits lead to the farms
    (`arefarm`, `elvfarm`) the port doesn't have yet;
  - City Hall: William moved to 44,43 and Kennedy added; three shopkeepers on `default`.

## Adding a map from the original files

```bash
go run ./server/tools/mapimport -src "<HelbreathServer-main>/Files/GameServers" \
    -maps assets/MAPDATA -only arefarm
```

It writes only maps that have an `.amd` in `assets/MAPDATA` and never overwrites an existing
config unless given `-force` (the configs are edited by hand once imported).
