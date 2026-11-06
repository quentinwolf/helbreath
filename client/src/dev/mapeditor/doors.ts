// Door data for the map editor: grouping door tiles into doors, the teleports.json
// format, and the checks behind the Problems list. No Phaser or DOM in here.

import type { SpawnSpot } from './spawns';

/** Where a door takes you. dir: 1 = north, clockwise to 8 = north-west; 0 keeps your facing. */
export interface Dest {
    map: string;
    x: number;
    y: number;
    dir: number;
}

/** A door tile as the Go export lists it. */
export interface ExportTile {
    x: number;
    y: number;
    to: { map: string; x: number; y: number; dir?: number };
    borrowed?: boolean;
}

/** One server map from the Go export. */
export interface ServerMapInfo {
    name: string;
    file: string;
    width: number;
    height: number;
    /** Tiles that work as doors today with the built-in doors. */
    doors: ExportTile[];
    /** Built-in door entries on tiles without the teleport flag (they never fire). */
    unflagged: ExportTile[];
    /** The built-in spawn list for this map. */
    spawns: SpawnSpot[];
}

export interface OverrideDoor {
    name?: string;
    tiles: [number, number][];
    to: { map: string; x: number; y: number; dir?: number };
}

export interface Overrides {
    version: number;
    maps: Record<string, OverrideDoor[]>;
}

export interface Door {
    id: number;
    name: string;
    /** "x,y" keys */
    tiles: Set<string>;
    to: Dest;
}

export const DIR_NAMES = ['keep facing', 'N', 'NE', 'E', 'SE', 'S', 'SW', 'W', 'NW'];
/** Tile step per direction (index = dir), as the server's dirDX/dirDY. */
export const DIR_DX = [0, 0, 1, 1, 1, 0, -1, -1, -1];
export const DIR_DY = [0, -1, -1, 0, 1, 1, 1, 0, -1];

let nextDoorId = 1;
export const newDoorId = (): number => nextDoorId++;

export const tileKey = (x: number, y: number): string => `${x},${y}`;
export const parseTileKey = (k: string): [number, number] => {
    const [x, y] = k.split(',').map(Number);
    return [x, y];
};

const destOf = (to: ExportTile['to']): Dest => ({ map: to.map, x: to.x, y: to.y, dir: to.dir ?? 0 });
const destKey = (d: Dest): string => `${d.map.toLowerCase()}|${d.x}|${d.y}|${d.dir}`;

/**
 * Groups door tiles into doors: tiles with the same destination up to 2 tiles
 * apart (one doorway, even with gaps) are one door.
 */
export function groupTiles(tiles: { x: number; y: number; to: Dest }[]): Door[] {
    const byDest = new Map<string, { to: Dest; keys: Set<string> }>();
    for (const t of tiles) {
        const k = destKey(t.to);
        if (!byDest.has(k)) byDest.set(k, { to: t.to, keys: new Set() });
        byDest.get(k)!.keys.add(tileKey(t.x, t.y));
    }
    const doors: Door[] = [];
    for (const { to, keys } of byDest.values()) {
        const left = new Set(keys);
        for (const start of keys) {
            if (!left.has(start)) continue;
            const group = new Set<string>([start]);
            left.delete(start);
            const queue = [start];
            while (queue.length) {
                const [x, y] = parseTileKey(queue.pop()!);
                for (let dy = -2; dy <= 2; dy++) {
                    for (let dx = -2; dx <= 2; dx++) {
                        const n = tileKey(x + dx, y + dy);
                        if (left.has(n)) {
                            left.delete(n);
                            group.add(n);
                            queue.push(n);
                        }
                    }
                }
            }
            doors.push({ id: newDoorId(), name: '', tiles: group, to: { ...to } });
        }
    }
    return sortDoors(doors);
}

/** Doors in reading order (top to bottom, then left to right) of their first tile. */
export function sortDoors(doors: Door[]): Door[] {
    const first = (d: Door) => {
        let best: [number, number] = [Infinity, Infinity];
        for (const k of d.tiles) {
            const [x, y] = parseTileKey(k);
            if (y < best[1] || (y === best[1] && x < best[0])) best = [x, y];
        }
        return best;
    };
    return doors.sort((a, b) => {
        const [ax, ay] = first(a);
        const [bx, by] = first(b);
        return ay - by || ax - bx;
    });
}

export function builtinDoors(info: ServerMapInfo): Door[] {
    return groupTiles(info.doors.map((t) => ({ x: t.x, y: t.y, to: destOf(t.to) })));
}

/** Built-in doors plus the unflagged entries, as if those worked too. */
export function builtinDoorsWithUnflagged(info: ServerMapInfo): Door[] {
    return groupTiles([...info.doors, ...info.unflagged].map((t) => ({ x: t.x, y: t.y, to: destOf(t.to) })));
}

export function doorsFromOverride(list: OverrideDoor[]): Door[] {
    return list.map((d) => ({
        id: newDoorId(),
        name: d.name ?? '',
        tiles: new Set(d.tiles.map(([x, y]) => tileKey(x, y))),
        to: destOf(d.to),
    }));
}

export function doorsToOverride(doors: Door[]): OverrideDoor[] {
    return doors
        .filter((d) => d.tiles.size > 0)
        .map((d) => {
            const tiles = [...d.tiles].map(parseTileKey).sort((a, b) => a[1] - b[1] || a[0] - b[0]);
            const to: OverrideDoor['to'] = { map: d.to.map, x: d.to.x, y: d.to.y };
            if (d.to.dir) to.dir = d.to.dir;
            const out: OverrideDoor = { tiles, to };
            if (d.name.trim()) out.name = d.name.trim();
            return out;
        });
}

export function doorLabel(d: Door): string {
    const where = `${d.to.map} (${d.to.x}, ${d.to.y})`;
    return d.name ? `${d.name} → ${where}` : `→ ${where}`;
}

/** Server tile flags for one map: 1 byte per tile. */
export interface TileFlags {
    width: number;
    height: number;
    /** Blocked as the server will see it: the .amd plus walkable.json edits. */
    blocked: Uint8Array;
    /** Blocked as in the .amd file. */
    origBlocked: Uint8Array;
    teleport: Uint8Array;
}

export interface WalkEdit {
    block?: [number, number][];
    open?: [number, number][];
}

export interface WalkOverrides {
    version: number;
    maps: Record<string, WalkEdit>;
}

/** Applies a map's walkable.json edit to its flags (tiles off the map are skipped). */
export function applyWalkEdit(f: TileFlags, e: WalkEdit): void {
    const set = (tiles: [number, number][] | undefined, v: number) => {
        for (const [x, y] of tiles ?? []) {
            if (x >= 0 && y >= 0 && x < f.width && y < f.height) f.blocked[y * f.width + x] = v;
        }
    };
    set(e.block, 1);
    set(e.open, 0);
}

/** The tiles whose walkability differs from the .amd, in walkable.json form. */
export function walkEditOf(f: TileFlags): WalkEdit {
    const block: [number, number][] = [];
    const open: [number, number][] = [];
    for (let i = 0; i < f.blocked.length; i++) {
        if (f.blocked[i] === f.origBlocked[i]) continue;
        const xy: [number, number] = [i % f.width, Math.floor(i / f.width)];
        (f.blocked[i] ? block : open).push(xy);
    }
    const e: WalkEdit = {};
    if (block.length) e.block = block;
    if (open.length) e.open = open;
    return e;
}

export function walkEditCount(f: TileFlags): number {
    let n = 0;
    for (let i = 0; i < f.blocked.length; i++) if (f.blocked[i] !== f.origBlocked[i]) n++;
    return n;
}

/** Tiles on a straight line from (x0, y0) to (x1, y1), so fast drags leave no gaps. */
export function lineTiles(x0: number, y0: number, x1: number, y1: number): [number, number][] {
    const out: [number, number][] = [];
    const dx = Math.abs(x1 - x0);
    const dy = -Math.abs(y1 - y0);
    const sx = x0 < x1 ? 1 : -1;
    const sy = y0 < y1 ? 1 : -1;
    let err = dx + dy;
    for (;;) {
        out.push([x0, y0]);
        if (x0 === x1 && y0 === y1) return out;
        const e2 = 2 * err;
        if (e2 >= dy) { err += dy; x0 += sx; }
        if (e2 <= dx) { err += dx; y0 += sy; }
    }
}

/** Reads the walk and teleport flags from a .amd file (same layout as HBMap / the Go loader). */
export function parseAmdFlags(buffer: ArrayBuffer): TileFlags {
    const bytes = new Uint8Array(buffer);
    const header = new TextDecoder('ascii').decode(bytes.slice(0, 256)).replace(/\0/g, ' ');
    const tokens = header.split(/\s+/).filter(Boolean);
    const value = (name: string) => parseInt(tokens[tokens.indexOf(name) + 2], 10);
    const width = value('MAPSIZEX');
    const height = value('MAPSIZEY');
    const tileSize = value('TILESIZE');
    if (!(width > 0 && height > 0 && tileSize >= 9)) throw new Error(`bad .amd header: ${header.trim()}`);
    const blocked = new Uint8Array(width * height);
    const teleport = new Uint8Array(width * height);
    for (let i = 0; i < width * height; i++) {
        const flags = bytes[256 + i * tileSize + 8];
        blocked[i] = flags & 0x80 ? 1 : 0;
        teleport[i] = flags & 0x40 ? 1 : 0;
    }
    return { width, height, blocked, origBlocked: blocked.slice(), teleport };
}

export function isBlocked(f: TileFlags, x: number, y: number): boolean {
    if (x < 0 || y < 0 || x >= f.width || y >= f.height) return true;
    return f.blocked[y * f.width + x] === 1;
}

export type Severity = 'error' | 'warn' | 'info';

export interface Problem {
    severity: Severity;
    text: string;
    /** Tile to centre on when clicked (on the current map). */
    x?: number;
    y?: number;
    doorId?: number;
    spawnId?: number;
    /** A one-click fix the editor offers. */
    fix?: 'enable-unflagged';
}

export interface ProblemContext {
    mapName: string;
    doors: Door[];
    flags: TileFlags;
    /** true: the map's doors come from teleports.json (exact tiles, flags ignored). */
    edited: boolean;
    info: ServerMapInfo;
    /** Server map name by lower-case name. */
    mapNames: Map<string, string>;
    /** Doors of another map (current working copy). */
    doorsOf: (map: string) => Door[];
    /** Flags of another map if already loaded. */
    flagsOf: (map: string) => TileFlags | undefined;
}

/** Everything worth knowing about the doors on one map. */
export function findProblems(c: ProblemContext): Problem[] {
    const out: Problem[] = [];
    const doorTiles = new Set<string>();
    for (const d of c.doors) for (const k of d.tiles) doorTiles.add(k);

    for (const d of c.doors) {
        const [fx, fy] = parseTileKey([...d.tiles][0] ?? '0,0');
        const label = doorLabel(d);
        const blocked = [...d.tiles].filter((k) => isBlocked(c.flags, ...parseTileKey(k)));
        if (blocked.length === d.tiles.size) {
            out.push({ severity: 'error', text: `${label}: every tile is blocked, so nobody can step on it`, x: fx, y: fy, doorId: d.id });
        } else if (blocked.length) {
            const [bx, by] = parseTileKey(blocked[0]);
            out.push({ severity: 'warn', text: `${label}: ${blocked.length} of ${d.tiles.size} tiles are blocked and never fire`, x: bx, y: by, doorId: d.id });
        }
        if (d.tiles.size === 1) {
            out.push({ severity: 'info', text: `${label}: only 1 tile wide, easy to walk past`, x: fx, y: fy, doorId: d.id });
        }

        const destName = c.mapNames.get(d.to.map.toLowerCase());
        if (!destName) {
            out.push({ severity: 'error', text: `${label}: the server has no map "${d.to.map}"`, x: fx, y: fy, doorId: d.id });
            continue;
        }
        if (destName !== d.to.map) {
            out.push({ severity: 'warn', text: `${label}: map is spelled "${destName}" on the server; the server needs the exact spelling`, x: fx, y: fy, doorId: d.id });
        }
        const destFlags = c.flagsOf(destName);
        if (destFlags) {
            if (d.to.x < 0 || d.to.y < 0) {
                out.push({ severity: 'info', text: `${label}: arrives at the map centre (x/y -1)`, x: fx, y: fy, doorId: d.id });
            } else if (isBlocked(destFlags, d.to.x, d.to.y)) {
                out.push({ severity: 'warn', text: `${label}: the arrival tile is blocked; the server moves you to the nearest open tile`, x: fx, y: fy, doorId: d.id });
            }
        }
        const destDoorTiles = new Set<string>();
        const destDoors = c.doorsOf(destName);
        for (const dd of destDoors) for (const k of dd.tiles) destDoorTiles.add(k);
        if (destDoorTiles.has(tileKey(d.to.x, d.to.y))) {
            out.push({ severity: 'warn', text: `${label}: you arrive standing on a door tile there`, x: fx, y: fy, doorId: d.id });
        }
        if (!destDoors.some((dd) => dd.to.map.toLowerCase() === c.mapName.toLowerCase())) {
            out.push({ severity: 'info', text: `${label}: ${destName} has no door back to ${c.mapName}`, x: fx, y: fy, doorId: d.id });
        }
    }

    if (!c.edited) {
        if (c.info.unflagged.length) {
            const t = c.info.unflagged[0];
            const dests = [...new Set(c.info.unflagged.map((u) => u.to.map))].join(', ');
            out.push({
                severity: 'error',
                text: `${c.info.unflagged.length} built-in door tiles never fire because the map has no teleport flag there (doors to ${dests})`,
                x: t.x, y: t.y, fix: 'enable-unflagged',
            });
        }
        let unused = 0;
        let ux = 0;
        let uy = 0;
        for (let y = 0; y < c.flags.height; y++) {
            for (let x = 0; x < c.flags.width; x++) {
                if (c.flags.teleport[y * c.flags.width + x] && !doorTiles.has(tileKey(x, y))) {
                    if (!unused) [ux, uy] = [x, y];
                    unused++;
                }
            }
        }
        if (unused) {
            out.push({ severity: 'info', text: `${unused} tiles have the teleport flag but no door within 2 tiles (they do nothing)`, x: ux, y: uy });
        }
    }

    const rank: Record<Severity, number> = { error: 0, warn: 1, info: 2 };
    return out.sort((a, b) => rank[a.severity] - rank[b.severity]);
}
