// Spawn data for the map editor: the save format (written into the map's
// maps/<map>.txt as spot-mob-generator and npc lines), working copies, and the
// checks behind the Problems list. No Phaser or DOM in here.
import { Door, Problem, TileFlags, isBlocked, tileKey } from './doors';

/** One spawn as the Go export has it (and as the editor saves it). */
export interface SpawnSpot {
    npc: number;
    count?: number;
    x: number;
    y: number;
    radius?: number;
    /** x1, y1, x2, y2: NPCs spawn, respawn and wander anywhere inside. */
    area?: [number, number, number, number];
    dir?: number;
    respawn?: number;
    note?: string;
    /** The map config line it came from ("spot:<num>" / "npc:<line>"), so saving rewrites that line. */
    src?: string;
}

export interface SpawnOverrides {
    version: number;
    maps: Record<string, SpawnSpot[]>;
}

export interface NpcTypeInfo {
    id: number;
    name: string;
    /** The name map configs use for it; missing = it can't be written to one. */
    configName?: string;
    stationary?: boolean;
    town?: boolean;
}

/** A spawn being edited. */
export interface Spawn {
    id: number;
    npc: number;
    count: number;
    x: number;
    y: number;
    radius: number;
    area: [number, number, number, number] | null;
    dir: number;
    respawn: number;
    note: string;
    src: string;
}

let nextSpawnId = 1;
export const newSpawnId = (): number => nextSpawnId++;

export function spawnFromSpot(s: SpawnSpot): Spawn {
    return {
        id: newSpawnId(),
        npc: s.npc,
        count: s.count ?? 0,
        x: s.x,
        y: s.y,
        radius: s.radius ?? 0,
        area: s.area ? [...s.area] as Spawn['area'] : null,
        dir: s.dir ?? 0,
        respawn: s.respawn ?? 0,
        note: s.note ?? '',
        src: s.src ?? '',
    };
}

export function spawnToSpot(s: Spawn): SpawnSpot {
    const out: SpawnSpot = { npc: s.npc, x: s.x, y: s.y };
    if (s.count > 1) out.count = s.count;
    if (s.area) {
        out.area = [...s.area] as SpawnSpot['area'];
        // The server ignores x/y for area spawns; keep them at the centre for readability.
        out.x = Math.floor((s.area[0] + s.area[2]) / 2);
        out.y = Math.floor((s.area[1] + s.area[3]) / 2);
    } else if (s.radius > 0) {
        out.radius = s.radius;
    }
    if (s.dir) out.dir = s.dir;
    if (s.respawn) out.respawn = s.respawn;
    if (s.note.trim()) out.note = s.note.trim();
    if (s.src) out.src = s.src;
    return out;
}

export const cloneSpawns = (list: Spawn[]): Spawn[] =>
    list.map((s) => ({ ...s, area: s.area ? [...s.area] as Spawn['area'] : null }));

/** Where the spawn is drawn: its point, or its area's centre. */
export function spawnAnchor(s: Spawn): { x: number; y: number } {
    if (s.area) return { x: Math.floor((s.area[0] + s.area[2]) / 2), y: Math.floor((s.area[1] + s.area[3]) / 2) };
    return { x: s.x, y: s.y };
}

export function spawnCount(s: Spawn): number {
    return Math.max(1, s.count);
}

export function spawnLabel(s: Spawn, types: Map<number, NpcTypeInfo>): string {
    const name = types.get(s.npc)?.name ?? `NPC ${s.npc}`;
    const n = spawnCount(s);
    return `${s.note ? `${s.note}: ` : ''}${name}${n > 1 ? ` ×${n}` : ''}`;
}

export type SpawnKind = 'town' | 'guard' | 'stationary' | 'monster';

export function spawnKind(t: NpcTypeInfo | undefined): SpawnKind {
    if (!t) return 'monster';
    if (t.town) return 'town';
    if (/guard/i.test(t.name)) return 'guard';
    if (t.stationary) return 'stationary';
    return 'monster';
}

export const KIND_COLOUR: Record<SpawnKind, number> = {
    town: 0x4fc3f7,
    guard: 0x81c784,
    stationary: 0xb39ddb,
    monster: 0xff7043,
};

export interface SpawnProblemContext {
    spawns: Spawn[];
    flags: TileFlags;
    doors: Door[];
    types: Map<number, NpcTypeInfo>;
}

export function findSpawnProblems(c: SpawnProblemContext): Problem[] {
    const out: Problem[] = [];
    const doorTiles = new Set<string>();
    for (const d of c.doors) for (const k of d.tiles) doorTiles.add(k);
    const f = c.flags;
    for (const s of c.spawns) {
        const label = spawnLabel(s, c.types);
        const at = spawnAnchor(s);
        const base = { x: at.x, y: at.y, spawnId: s.id };
        if (!c.types.has(s.npc)) {
            out.push({ severity: 'error', text: `${label}: unknown NPC type ${s.npc}`, ...base });
        } else if (!c.types.get(s.npc)!.configName) {
            out.push({ severity: 'error', text: `${label}: this NPC type shares its name with others and can't be saved in a map config; pick another`, ...base });
        }
        if (s.area) {
            const [x1, y1, x2, y2] = s.area;
            let open = 0;
            let total = 0;
            for (let y = y1; y <= y2; y++) {
                for (let x = x1; x <= x2; x++) {
                    total++;
                    if (!isBlocked(f, x, y)) open++;
                }
            }
            if (x2 < 0 || y2 < 0 || x1 >= f.width || y1 >= f.height) {
                out.push({ severity: 'error', text: `${label}: the area is outside the map`, ...base });
            } else if (open === 0) {
                out.push({ severity: 'error', text: `${label}: every tile in the area is blocked`, ...base });
            } else if (open < spawnCount(s)) {
                out.push({ severity: 'warn', text: `${label}: only ${open} open tiles in the area for ${spawnCount(s)} NPCs`, ...base });
            } else if (open < total / 4) {
                out.push({ severity: 'info', text: `${label}: most of the area is blocked (${open} of ${total} tiles open)`, ...base });
            }
            continue;
        }
        if (s.x < 0 || s.y < 0 || s.x >= f.width || s.y >= f.height) {
            out.push({ severity: 'error', text: `${label}: the spot is outside the map`, ...base });
        } else if (isBlocked(f, s.x, s.y) && s.radius === 0) {
            out.push({ severity: 'warn', text: `${label}: the spot is blocked; the server moves it to the nearest open tile`, ...base });
        }
        if (doorTiles.has(tileKey(s.x, s.y)) && s.radius === 0) {
            out.push({ severity: 'warn', text: `${label}: stands on a door tile`, ...base });
        }
    }
    return out;
}
