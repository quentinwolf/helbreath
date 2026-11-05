import type { Scene } from 'phaser';
import { EFFECT_POISON_CLOUD, getEffectByKey, type EffectConfig } from '../../constants/Effects';
import { Effect } from '../effects/Effect';

export type PoisonCloudInstanceConfig = {
    /** Milliseconds the cloud lingers before it starts dissipating (default: 30000) */
    duration?: number;
    /** Called when an effect is created. Returns onDestroy callback to remove from effects array. */
    onEffectCreated?: (effect: Effect) => () => void;
};

const DEFAULT_DURATION_MS = 30000;

// The original's poison cloud is a dynamic object with three phases, all on
// effect4.spr sheet 4: PCLOUD_BEGIN frames 0-7, PCLOUD_LOOP 8-15, PCLOUD_END 16-23.
const BEGIN: [number, number] = [0, 7];
const LOOP: [number, number] = [8, 15];
const END: [number, number] = [16, 23];

/**
 * One tile of Poison Cloud: billows up, churns for the spell's duration, then fades out.
 * Adapted from ErkoKnoll's helbreath-base-game (MIT), which looped all 24 frames.
 */
export class PoisonCloudInstance {
    private scene: Scene;
    private config: PoisonCloudInstanceConfig | undefined;
    private base: EffectConfig | undefined;
    private effect: Effect | undefined;
    private unregister: (() => void) | undefined;
    private endTimer: Phaser.Time.TimerEvent | undefined;
    private pixelX: number;
    private pixelY: number;
    private destroyed = false;

    constructor(scene: Scene, pixelX: number, pixelY: number, config?: PoisonCloudInstanceConfig) {
        this.scene = scene;
        this.config = config;
        this.pixelX = pixelX;
        this.pixelY = pixelY;
        this.base = getEffectByKey(EFFECT_POISON_CLOUD);
        if (!this.base) return;

        // Neighbouring tiles start a frame or two apart so the cloud doesn't pulse in sync.
        const stagger = Phaser.Math.Between(0, 150);
        this.scene.time.delayedCall(stagger, () => {
            if (this.destroyed) return;
            this.play(BEGIN, false, () => this.play(LOOP, true));
            this.endTimer = this.scene.time.delayedCall(config?.duration ?? DEFAULT_DURATION_MS, () => {
                this.play(END, false, () => this.destroy());
            });
        });
    }

    private play(frames: [number, number], loop: boolean, onDone?: () => void): void {
        if (this.destroyed || !this.base) return;
        this.clearEffect();
        const effect = new Effect(this.scene, {
            config: { ...this.base, animationFrames: frames },
            pixelX: this.pixelX,
            pixelY: this.pixelY,
            infiniteLoop: loop,
            onGround: true,
            onDestroy: () => {
                if (this.effect !== effect) return;
                this.effect = undefined;
                this.unregister?.();
                this.unregister = undefined;
                onDone?.();
            },
        });
        this.effect = effect;
        this.unregister = this.config?.onEffectCreated?.(effect);
    }

    private clearEffect(): void {
        const old = this.effect;
        this.effect = undefined;
        this.unregister?.();
        this.unregister = undefined;
        old?.destroy();
    }

    public destroy(): void {
        if (this.destroyed) return;
        this.destroyed = true;
        this.endTimer?.destroy();
        this.endTimer = undefined;
        this.clearEffect();
    }
}
