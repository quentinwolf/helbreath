import type { Scene } from 'phaser';
import type { SoundManager } from '../../audio/SoundManager';
import type { CameraManager } from '../../utils/CameraShake';
import { EFFECT_CAST, ENERGY_BOLT_LAUNCH_SOUND, MAGIC_MISSILE_EXPLOSION_SOUND } from '../../constants/EffectSounds';
import { calculateSpatialAudio } from '../../utils/SpatialAudioUtils';
import { convertPixelPosToWorldPos } from '../../utils/CoordinateUtils';
import { LightningBlast } from './LightningBlast';
import { PivotSprite, runFrames, rgbBrightness } from './OriginalFx';

// Ported from the original client (bAddNewEffect / bEffectFrameCounter / DrawEffects):
//   100 Magic Missile, 137 Lightning Arrow, 156 Mass Lightning Arrow (3 arrows),
//   the "magic lands on you" flashes 102 (effect.spr sheet 4: Poison, Detect
//   Invisibility, Polymorph, Cure, Paralyze, Confusion...) and 112 (effect7 sheet 7:
//   Recall, Summon Creature, Invisibility).
// Pixel positions are tile anchors (tile * 32), like character sprites.

export type MissileConfig = {
    soundManager?: SoundManager;
    playerWorldX?: number;
    playerWorldY?: number;
    cameraManager?: CameraManager;
};

const rand = (n: number) => Math.floor(Math.random() * n);

function play(config: MissileConfig, key: string, x: number, y: number): void {
    const { soundManager, playerWorldX, playerWorldY } = config;
    if (!soundManager || playerWorldX === undefined || playerWorldY === undefined) return;
    soundManager.playOnce(key, undefined, calculateSpatialAudio({
        sourceX: convertPixelPosToWorldPos(x),
        sourceY: convertPixelPosToWorldPos(y),
        listenerX: playerWorldX,
        listenerY: playerWorldY,
    }));
}

/** cCalcDirection: 1 = N, clockwise to 8 = NW. */
function calcDirection(sx: number, sy: number, dx: number, dy: number): number {
    const angle = Math.atan2(dy - sy, dx - sx) * 180 / Math.PI; // 0 = east, 90 = south
    return ((Math.round((angle + 90) / 45) % 8) + 8) % 8 + 1;
}

/** Effect 8: a spark that runs effect2 sheet 1 frames 4..0 (30 ms each). */
function spark(scene: Scene, x: number, y: number): void {
    const s = new PivotSprite(scene, 'effect2', 1);
    s.setEffectDepth(y, 1);
    runFrames(scene, 30, -rand(4), 4, (f) => s.draw(4 - f, x, y), () => s.destroy());
}

/**
 * Flies a projectile from (sx, sy - 40) to (dx, dy) at the original's 50 px per 20 ms
 * frame. `draw` renders it each frame (with its previous position for trails),
 * `trail` runs after each step, `land` at the target.
 */
function fly(
    scene: Scene, sx: number, sy: number, dx: number, dy: number,
    draw: (x: number, y: number, px: number, py: number) => void,
    trail: (x: number, y: number) => void,
    land: () => void,
): void {
    let x = sx;
    let y = sy - 40;
    let cancel = () => {};
    let done = false;
    draw(x, y, x, y);
    cancel = runFrames(scene, 20, 1, 10000, () => {
        if (done) return;
        const px = x;
        const py = y;
        const dist = Math.max(Math.abs(dx - x), Math.abs(dy - y));
        const t = dist <= 50 ? 1 : 50 / dist;
        x = Math.round(x + (dx - x) * t);
        y = Math.round(y + (dy - y) * t);
        trail(x, y);
        draw(x, y, px, py);
        if (Math.abs(x - dx) <= 2 && Math.abs(y - dy) <= 2) {
            done = true;
            cancel();
            land();
        }
    }, () => {});
}

/** Magic Missile (spell 0): a bright orb trailing sparks; a small blast on landing. */
export function castMagicMissile(scene: Scene, sx: number, sy: number, dx: number, dy: number, config: MissileConfig): void {
    play(config, ENERGY_BOLT_LAUNCH_SOUND, sx, sy);
    const orb = new PivotSprite(scene, 'effect', 0);
    fly(scene, sx, sy, dx, dy,
        (x, y) => { orb.draw(0, x, y); orb.setEffectDepth(y, 5); },
        (x, y) => spark(scene, x + rand(20) - 10, y + rand(20) - 10),
        () => {
            orb.destroy();
            // Effect 7: effect.spr sheet 6 frames 0-5 (50 ms), darkening from frame 4
            const blast = new PivotSprite(scene, 'effect', 6);
            blast.setEffectDepth(dy, 5);
            play(config, MAGIC_MISSILE_EXPLOSION_SOUND, dx, dy);
            runFrames(scene, 50, 0, 5, (f) => blast.draw(f, dx, dy, true, f < 4 ? 1 : rgbBrightness((f - 4) * -3)),
                () => blast.destroy());
        });
}

/** Lightning Arrow (spell 37): a directional bolt with a fading trail and sparks. */
export function castLightningArrow(scene: Scene, sx: number, sy: number, dx: number, dy: number, config: MissileConfig): void {
    play(config, ENERGY_BOLT_LAUNCH_SOUND, sx, sy);
    const base = (calcDirection(sx, sy, dx, dy) - 1) * 4;
    // The head plus five trail copies 15-75 px back, at 70/50/50/25/25% (DrawEffects 137)
    const trailAlpha = [0.7, 0.5, 0.5, 0.25, 0.25];
    const parts = [new PivotSprite(scene, 'effect2', 0), ...trailAlpha.map(() => new PivotSprite(scene, 'effect2', 0))];
    fly(scene, sx, sy, dx, dy,
        (x, y, px, py) => {
            parts[0].draw(base + rand(4), x, y);
            parts[0].setEffectDepth(y, 6);
            const back = Math.max(Math.abs(px - x), Math.abs(py - y)) || 1;
            trailAlpha.forEach((alpha, i) => {
                const t = Math.min(1, (15 * (i + 1)) / back);
                const tx = Math.round(x + (px - x) * t);
                const ty = Math.round(y + (py - y) * t);
                parts[i + 1].draw(base + rand(4), tx, ty, true, alpha);
                parts[i + 1].setEffectDepth(y, 5);
            });
        },
        (x, y) => {
            for (let i = 0; i < 3; i++) spark(scene, x + rand(20) - 10, y + rand(20) - 10);
        },
        () => {
            parts.forEach((p) => p.destroy());
            LightningBlast.spawn(scene, convertPixelPosToWorldPos(dx), convertPixelPosToWorldPos(dy), config);
        });
}

/** Mass Lightning Arrow (spell 56): three Lightning Arrows, 130 ms apart. */
export function castMassLightningArrow(scene: Scene, sx: number, sy: number, dx: number, dy: number, config: MissileConfig): void {
    runFrames(scene, 130, 0, 3, (f) => {
        if (f > 0) castLightningArrow(scene, sx, sy, dx, dy, config);
    }, () => {});
}

/**
 * Effect 102: the swirl on the target of Poison, Cure, Detect Invisibility, Polymorph,
 * Paralyze, Confusion and the like. effect.spr sheet 4, 13 frames at 120 ms, darkening
 * from frame 5.
 */
export function castMagicFlash(scene: Scene, x: number, y: number, config: MissileConfig): void {
    play(config, EFFECT_CAST, x, y);
    const s = new PivotSprite(scene, 'effect', 4);
    s.setEffectDepth(y, 5);
    runFrames(scene, 120, 0, 13, (f) => s.draw(f, x, y, true, f < 5 ? 1 : rgbBrightness((f - 5) * -5)),
        () => s.destroy());
}

/** Effect 112: Recall, Summon Creature and Invisibility. effect7 sheet 7, 12 frames at 80 ms. */
export function castTransportFlash(scene: Scene, x: number, y: number, config: MissileConfig): void {
    play(config, EFFECT_CAST, x, y);
    const s = new PivotSprite(scene, 'effect7', 7);
    s.setEffectDepth(y, 5);
    runFrames(scene, 80, 0, 12, (f) => s.draw(f, x, y), () => s.destroy());
}
