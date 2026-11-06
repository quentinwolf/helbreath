// Dev-server API for the map editor (client/map-editor.html, docs/map-editor.md).
// Only runs under `npm run dev`; nothing here is part of the game build.
//
//   GET  /__map-editor/state               server maps + built-in doors, teleports.json, client map files
//   GET  /__map-editor/server-map?file=X   a .amd file from the server's map folder (assets/MAPDATA)
//   POST /__map-editor/save                writes teleports.json, walkable.json and spawns.json (server/internal/mapdata)
import type { Plugin } from 'vite';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { execFile } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const PREFIX = '/__map-editor/';

interface OverrideDoor {
    name?: string;
    tiles: [number, number][];
    to: { map: string; x: number; y: number; dir?: number };
}
interface Overrides {
    version: number;
    maps: Record<string, OverrideDoor[]>;
}
interface SpawnOverrides {
    version: number;
    maps: Record<string, Record<string, unknown>[]>;
}
interface WalkOverrides {
    version: number;
    maps: Record<string, { block?: [number, number][]; open?: [number, number][] }>;
}

export function mapEditorApi(): Plugin {
    let repo = '';
    let exportCache: string | null = null;

    const paths = () => ({
        serverMaps: path.join(repo, 'assets', 'MAPDATA'),
        clientMaps: path.join(repo, 'client', 'public', 'assets', 'maps'),
        overrides: path.join(repo, 'server', 'internal', 'mapdata', 'teleports.json'),
        walkable: path.join(repo, 'server', 'internal', 'mapdata', 'walkable.json'),
        spawns: path.join(repo, 'server', 'internal', 'mapdata', 'spawns.json'),
    });

    const goBinary = (): string => {
        if (process.env.GO_BIN) return process.env.GO_BIN;
        const windowsDefault = 'C:\\Program Files\\Go\\bin\\go.exe';
        return process.platform === 'win32' && fs.existsSync(windowsDefault) ? windowsDefault : 'go';
    };

    // The Go tool resolves the built-in doors exactly as the server does.
    const runExport = (): Promise<string> =>
        new Promise((resolve, reject) => {
            execFile(goBinary(), ['run', './server/tools/teleportexport', '-maps', paths().serverMaps],
                { cwd: repo, maxBuffer: 64 * 1024 * 1024 },
                (err, stdout, stderr) => {
                    if (err) reject(new Error(`teleportexport failed: ${stderr || err.message}`));
                    else resolve(stdout);
                });
        });

    const send = (res: ServerResponse, status: number, body: unknown) => {
        res.statusCode = status;
        res.setHeader('Content-Type', 'application/json');
        res.end(JSON.stringify(body));
    };

    const readBody = (req: IncomingMessage): Promise<string> =>
        new Promise((resolve, reject) => {
            let data = '';
            req.setEncoding('utf8');
            req.on('data', (chunk: string) => { data += chunk; });
            req.on('end', () => resolve(data));
            req.on('error', reject);
        });

    const state = async (refresh: boolean) => {
        if (refresh || exportCache === null) exportCache = await runExport();
        const exported = JSON.parse(exportCache);
        const p = paths();
        const overrides = JSON.parse(fs.readFileSync(p.overrides, 'utf8')) as Overrides;
        const walkable = JSON.parse(fs.readFileSync(p.walkable, 'utf8')) as WalkOverrides;
        const spawns = JSON.parse(fs.readFileSync(p.spawns, 'utf8')) as SpawnOverrides;
        const clientMaps = fs.readdirSync(p.clientMaps).filter((f) => f.toLowerCase().endsWith('.amd'));
        return { ...exported, overrides, walkable, spawns, clientMaps };
    };

    return {
        name: 'hb-map-editor-api',
        apply: 'serve',
        configResolved(config) {
            repo = path.resolve(config.root, '..');
        },
        configureServer(server) {
            server.middlewares.use(async (req, res, next) => {
                if (!req.url?.startsWith(PREFIX)) return next();
                const url = new URL(req.url, 'http://localhost');
                const route = url.pathname.slice(PREFIX.length);
                try {
                    if (route === 'state' && req.method === 'GET') {
                        send(res, 200, await state(url.searchParams.has('refresh')));
                    } else if (route === 'server-map' && req.method === 'GET') {
                        const file = url.searchParams.get('file') ?? '';
                        const dir = paths().serverMaps;
                        const match = fs.readdirSync(dir).find((f) => f.toLowerCase() === file.toLowerCase());
                        if (!match || path.basename(file) !== file) return send(res, 404, { error: `no server map ${file}` });
                        res.setHeader('Content-Type', 'application/octet-stream');
                        res.end(fs.readFileSync(path.join(dir, match)));
                    } else if (route === 'save' && req.method === 'POST') {
                        const body = JSON.parse(await readBody(req)) as { teleports?: Overrides; walkable?: WalkOverrides; spawns?: SpawnOverrides };
                        const valid = (o?: { version: number; maps: unknown }) =>
                            o?.version === 1 && typeof o.maps === 'object' && o.maps !== null;
                        if ([body.teleports, body.walkable, body.spawns].some((o) => o && !valid(o))) {
                            return send(res, 400, { error: 'files need version 1 and maps' });
                        }
                        // The game server reloads these files when they change, so write each
                        // in one step (it never sees half a file).
                        if (body.teleports) writeWhole(paths().overrides, formatOverrides(body.teleports));
                        if (body.walkable) writeWhole(paths().walkable, formatWalkable(body.walkable));
                        if (body.spawns) writeWhole(paths().spawns, formatSpawns(body.spawns));
                        // Re-run the Go tool: it parses the new files the way the server will.
                        const after = await state(true);
                        send(res, 200, {
                            ok: !after.overrideError && !after.walkError && !after.spawnError,
                            overrideError: after.overrideError ?? '',
                            walkError: after.walkError ?? '',
                            spawnError: after.spawnError ?? '',
                        });
                    } else {
                        send(res, 404, { error: `unknown map editor route ${route}` });
                    }
                } catch (err) {
                    send(res, 500, { error: err instanceof Error ? err.message : String(err) });
                }
            });
        },
    };
}

/** Writes a temp file and renames it over the target (plain write if the rename fails). */
function writeWhole(file: string, text: string): void {
    const tmp = `${file}.tmp`;
    fs.writeFileSync(tmp, text);
    try {
        fs.renameSync(tmp, file);
    } catch {
        fs.writeFileSync(file, text);
        fs.rmSync(tmp, { force: true });
    }
}

/** teleports.json with one door per line, maps sorted, so diffs stay readable. */
export function formatOverrides(o: Overrides): string {
    const names = Object.keys(o.maps).sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    if (names.length === 0) return '{\n  "version": 1,\n  "maps": {}\n}\n';
    const mapBlocks = names.map((name) => {
        const doors = o.maps[name].map((d) => {
            const door: OverrideDoor = d.name ? { name: d.name, to: d.to, tiles: d.tiles } : { to: d.to, tiles: d.tiles };
            return `      ${JSON.stringify(door)}`;
        });
        return `    ${JSON.stringify(name)}: [\n${doors.join(',\n')}\n    ]`;
    });
    return `{\n  "version": 1,\n  "maps": {\n${mapBlocks.join(',\n')}\n  }\n}\n`;
}

/** walkable.json with one line per map and list, maps sorted. */
export function formatWalkable(o: WalkOverrides): string {
    const names = Object.keys(o.maps)
        .filter((m) => o.maps[m].block?.length || o.maps[m].open?.length)
        .sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    if (names.length === 0) return '{\n  "version": 1,\n  "maps": {}\n}\n';
    const blocks = names.map((name) => {
        const e = o.maps[name];
        const lists = (['block', 'open'] as const)
            .filter((k) => e[k]?.length)
            .map((k) => `      ${JSON.stringify(k)}: ${JSON.stringify(e[k])}`);
        return `    ${JSON.stringify(name)}: {\n${lists.join(',\n')}\n    }`;
    });
    return `{\n  "version": 1,\n  "maps": {\n${blocks.join(',\n')}\n  }\n}\n`;
}

/** spawns.json with one spawn per line, maps sorted. */
export function formatSpawns(o: SpawnOverrides): string {
    const names = Object.keys(o.maps).sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
    if (names.length === 0) return '{\n  "version": 1,\n  "maps": {}\n}\n';
    const blocks = names.map((name) => {
        const spots = o.maps[name].map((s) => `      ${JSON.stringify(s)}`);
        return `    ${JSON.stringify(name)}: [${spots.length ? `\n${spots.join(',\n')}\n    ` : ''}]`;
    });
    return `{\n  "version": 1,\n  "maps": {\n${blocks.join(',\n')}\n  }\n}\n`;
}
