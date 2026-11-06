import type { Scene } from 'phaser';
import type { SoundManager } from '../../audio/SoundManager';
import type { CameraManager } from '../../utils/CameraShake';
import { EFFECT_EXPLOSION, FIREWORK_SOUND } from '../../constants/EffectSounds';
import { calculateSpatialAudio } from '../../utils/SpatialAudioUtils';
import { convertPixelPosToWorldPos } from '../../utils/CoordinateUtils';
import { PivotSprite, runFrames } from './OriginalFx';

// Spell effects ported from the original client's bAddNewEffect / DrawEffects
// (Client.cpp) that had no animation here yet. (x, y) is the target tile anchor.

export type OriginalFxConfig = {
    soundManager?: SoundManager;
    playerWorldX?: number;
    playerWorldY?: number;
    cameraManager?: CameraManager;
};

const rand = (n: number) => Math.floor(Math.random() * n);

function play(config: OriginalFxConfig, key: string, x: number, y: number): void {
    const { soundManager, playerWorldX, playerWorldY } = config;
    if (!soundManager || playerWorldX === undefined || playerWorldY === undefined) return;
    soundManager.playOnce(key, undefined, calculateSpatialAudio({
        sourceX: convertPixelPosToWorldPos(x),
        sourceY: convertPixelPosToWorldPos(y),
        listenerX: playerWorldX,
        listenerY: playerWorldY,
    }));
}

/**
 * Tremor (effect 138): sound E4, a camera shake, and 14 puffs of dust (effect 14:
 * effect2 sheet 1 frames 28-32 at 100 ms, Trans50) scattered 60 px across and 40 px
 * up/down around the target.
 */
export function castTremor(scene: Scene, x: number, y: number, config: OriginalFxConfig): void {
    play(config, EFFECT_EXPLOSION, x, y);
    config.cameraManager?.setCameraShake(x, y, 2);
    for (let i = 0; i < 14; i++) {
        const px = x + rand(120) - 60;
        const py = y + rand(80) - 40;
        const s = new PivotSprite(scene, 'effect2', 1);
        s.setEffectDepth(py);
        runFrames(scene, 100, 0, 4, (f) => s.draw(28 + f, px, py, true, 0.5), () => s.destroy());
    }
}

/**
 * Celebrating Light (effect 114): five fireworks (effects 69/70: effect6 sheet 2 or 3,
 * 11 frames at 30 ms, additive) within 20 px of the target, each starting 3 frames
 * after the last, with sound E42.
 */
export function castCelebratingLight(scene: Scene, x: number, y: number, config: OriginalFxConfig): void {
    play(config, FIREWORK_SOUND, x, y);
    for (const delay of [-12, -9, -6, -3, 0]) {
        const px = x + 20 - rand(40);
        const py = y + 20 - rand(40);
        const s = new PivotSprite(scene, 'effect6', 2 + rand(2));
        s.setEffectDepth(py);
        runFrames(scene, 30, delay, 11, (f) => s.draw(f, px, py), () => s.destroy());
    }
}
