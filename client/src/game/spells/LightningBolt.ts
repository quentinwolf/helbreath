import type { Scene } from 'phaser';
import { DEPTH_MULTIPLIER, EFFECT_LAYER_DEPTH } from '../../Config';
import { LIGHTNING_SOUND } from '../../constants/EffectSounds';
import type { SoundManager } from '../../audio/SoundManager';
import type { CameraManager } from '../../utils/CameraShake';
import { calculateSpatialAudio } from '../../utils/SpatialAudioUtils';
import { convertPixelPosToWorldPos } from '../../utils/CoordinateUtils';
import { LightningBlast } from './LightningBlast';
import { runFrames } from './OriginalFx';

// Lightning (spell 43, effect 143) and Lightning Bolt (51, effect 151), ported from the
// original client: each 10 ms frame redraws three _DrawThunderEffect strokes with a fresh
// random wobble, then an energy blast (effect 10) lands on the target.

export type LightningBoltConfig = {
    soundManager?: SoundManager;
    playerWorldX?: number;
    playerWorldY?: number;
    cameraManager?: CameraManager;
    /** Bolt start in pixels. Omitted: strikes from 800 px above the target (Lightning). */
    originPixelX?: number;
    originPixelY?: number;
};

// Colours of the original's additive line passes (8-bit RGB).
const CORE = 0x7b7ba5; // DrawLine(…, 15, 15, 20)
const GLOW1 = 0x323264; // RGB(50, 50, 100), 1 px either side
const GLOW2 = 0x1e1e64; // RGB(30, 30, 100), 2 px either side
const EDGE = 0x00001e; // RGB(0, 0, 30), diagonals
const THIN = 0x3232c8; // RGB(50, 50, 200), the two thin side strokes

const rand = (n: number) => Math.floor(Math.random() * n);

/** cGetNextMoveDir: 1 = N, clockwise to 8 = NW; 0 when already there. */
function nextMoveDir(x: number, y: number, tx: number, ty: number): number {
    const dx = Math.sign(tx - x);
    const dy = Math.sign(ty - y);
    if (dx === 0 && dy === 0) return 0;
    if (dx === 0) return dy < 0 ? 1 : 5;
    if (dy === 0) return dx > 0 ? 3 : 7;
    if (dx > 0) return dy < 0 ? 2 : 4;
    return dy < 0 ? 8 : 6;
}

/**
 * The path _DrawThunderEffect walks from (sx, sy) toward (dx, dy): it chases a point
 * moving 10 px per step along the straight line, steering its velocity by 5 px per
 * step (capped at 20), which gives the bolt its jagged shape.
 */
export function thunderPath(sx: number, sy: number, dx: number, dy: number, rX: number, rY: number): { x: number; y: number }[] {
    const points = [{ x: sx, y: sy }];
    const len = Math.max(Math.abs(dx - sx), Math.abs(dy - sy)) || 1;
    let x = sx;
    let y = sy;
    for (let j = 0; j < 100; j++) {
        const t = Math.min(1, (j * 10) / len);
        const tx = Math.round(sx + (dx - sx) * t);
        const ty = Math.round(sy + (dy - sy) * t);
        switch (nextMoveDir(x, y, tx, ty)) {
            case 1: rY -= 5; break;
            case 2: rY -= 5; rX += 5; break;
            case 3: rX += 5; break;
            case 4: rX += 5; rY += 5; break;
            case 5: rY += 5; break;
            case 6: rX -= 5; rY += 5; break;
            case 7: rX -= 5; break;
            case 8: rX -= 5; rY -= 5; break;
        }
        rX = Math.max(-20, Math.min(20, rX));
        rY = Math.max(-20, Math.min(20, rY));
        x += rX;
        y += rY;
        points.push({ x, y });
        if (Math.abs(tx - dx) < 5 && Math.abs(ty - dy) < 5) break;
    }
    return points;
}

export class LightningBolt {
    private graphics: Phaser.GameObjects.Graphics;
    private readonly fromSky: boolean;
    private readonly sx: number;
    private readonly sy: number;

    constructor(
        private scene: Scene,
        _originPixelX: number,
        _originPixelY: number,
        private targetPixelX: number,
        private targetPixelY: number,
        private config: LightningBoltConfig,
    ) {
        this.fromSky = config.originPixelX === undefined || config.originPixelY === undefined;
        this.sx = this.fromSky ? targetPixelX : config.originPixelX!;
        this.sy = this.fromSky ? targetPixelY - 800 : config.originPixelY!;

        this.graphics = scene.add.graphics();
        this.graphics.setBlendMode(Phaser.BlendModes.ADD);
        this.graphics.setDepth(EFFECT_LAYER_DEPTH + convertPixelPosToWorldPos(targetPixelY) * DEPTH_MULTIPLIER + 50);

        this.playThunderSound();
        runFrames(scene, 10, 0, this.fromSky ? 7 : 10, () => this.draw(), () => {
            this.graphics.destroy();
            LightningBlast.spawn(scene, convertPixelPosToWorldPos(targetPixelX), convertPixelPosToWorldPos(targetPixelY), {
                soundManager: config.soundManager,
                playerWorldX: config.playerWorldX,
                playerWorldY: config.playerWorldY,
                cameraManager: config.cameraManager,
            });
        });
    }

    private playThunderSound(): void {
        const { soundManager, playerWorldX, playerWorldY } = this.config;
        if (!soundManager || playerWorldX === undefined || playerWorldY === undefined) return;
        soundManager.playOnce(LIGHTNING_SOUND, undefined, calculateSpatialAudio({
            sourceX: convertPixelPosToWorldPos(this.targetPixelX),
            sourceY: convertPixelPosToWorldPos(this.targetPixelY),
            listenerX: playerWorldX,
            listenerY: playerWorldY,
        }));
    }

    private draw(): void {
        const g = this.graphics;
        g.clear();
        const rX = 5 - rand(10);
        const rY = 5 - rand(10);
        const { sx, sy, targetPixelX: dx, targetPixelY: dy } = this;
        this.stroke(thunderPath(sx, sy, dx, dy, rX, rY), true);
        if (this.fromSky) {
            this.stroke(thunderPath(sx, sy, dx, dy, rX + 4, rY + 2), false);
        } else {
            this.stroke(thunderPath(sx, sy, dx, dy, rX + 2, rY - 2), false);
        }
        this.stroke(thunderPath(sx, sy, dx, dy, rX - 2, rY - 2), false);
    }

    /** One stroke: the thick glowing core (type 1) or a thin blue line (type 2). */
    private stroke(path: { x: number; y: number }[], thick: boolean): void {
        const g = this.graphics;
        const pass = (color: number, ox: number, oy: number) => {
            g.lineStyle(1, color, 1);
            g.beginPath();
            g.moveTo(path[0].x + ox, path[0].y + oy);
            for (let i = 1; i < path.length; i++) g.lineTo(path[i].x + ox, path[i].y + oy);
            g.strokePath();
        };
        if (!thick) {
            pass(THIN, 0, 0);
            return;
        }
        pass(CORE, 0, 0);
        for (const [ox, oy] of [[-1, 0], [1, 0], [0, -1], [0, 1]]) pass(GLOW1, ox, oy);
        for (const [ox, oy] of [[-2, 0], [2, 0], [0, -2], [0, 2]]) pass(GLOW2, ox, oy);
        for (const [ox, oy] of [[-1, -1], [1, -1], [-1, 1]]) pass(EDGE, ox, oy);
    }

    public destroy(): void {
        this.graphics.destroy();
    }
}
