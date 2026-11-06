import type { Scene } from 'phaser';
import { PivotSprite } from '../spells/OriginalFx';

// Rain ported from the original client (Client.cpp WhetherObjectFrameCounter /
// DrawWhetherEffects). Each drop is effect2.spr sheet 1: it falls for 20 steps of
// 30 ms (40, 39 ... 21 px a step, 1 px left each step) drawn with frames 16-19,
// then splashes for 5 steps (frames 20-24) where it landed, then respawns above
// the screen after a short random delay. Drops live in world space, so they move
// with the map when you walk, and they land all over the screen, which makes the
// rain thin out towards the bottom.

export const RAIN_SPRITE = 'effect2';
const SHEET = 1;
const STEP_MS = 30;
const FALL_STEPS = 20;
const SPLASH_END = 25;
const DEPTH = 999990;

interface Drop {
    x: number;
    y: number;
    step: number;
    sprite: PivotSprite;
}

export class OriginalRain {
    private readonly scene: Scene;
    private readonly drops: Drop[] = [];
    private elapsed = 0;

    constructor(scene: Scene, count: number) {
        this.scene = scene;
        for (let i = 0; i < count; i++) {
            const sprite = new PivotSprite(scene, RAIN_SPRITE, SHEET);
            sprite.sprite.setDepth(DEPTH);
            const drop: Drop = { x: 0, y: 0, step: 0, sprite };
            // Original SetWhetherStatus: each drop starts up to 40 steps late
            this.respawn(drop, -Math.floor(Math.random() * 40));
            this.drops.push(drop);
        }
    }

    update(delta: number): void {
        this.elapsed += delta;
        // Catch up after a slow frame, but not after a long pause (tab hidden)
        const steps = Math.min(Math.floor(this.elapsed / STEP_MS), 5);
        this.elapsed = steps === 5 ? 0 : this.elapsed - steps * STEP_MS;
        for (let s = 0; s < steps; s++) this.tick();
        for (const drop of this.drops) this.draw(drop);
    }

    destroy(): void {
        for (const drop of this.drops) drop.sprite.destroy();
        this.drops.length = 0;
    }

    private tick(): void {
        for (const drop of this.drops) {
            drop.step++;
            if (drop.step >= 0 && drop.step < FALL_STEPS) {
                drop.y += 40 - drop.step;
                drop.x -= 1;
            } else if (drop.step >= SPLASH_END) {
                this.respawn(drop, -Math.floor(Math.random() * 10));
            }
        }
    }

    /**
     * The original spawned drops from 92 px left of the view to 848 px right of it
     * (800 px wide screen), and from 616 px above it to 184 px below it, so after
     * their 610 px fall they land anywhere from the top of the screen to a bit
     * below the bottom. Same spread here, sized to our camera.
     */
    private respawn(drop: Drop, step: number): void {
        const cam = this.scene.cameras.main;
        drop.x = cam.scrollX - 100 + Math.random() * (cam.width + 140);
        drop.y = cam.scrollY - 616 + Math.random() * (cam.height + 200);
        drop.step = step;
    }

    private draw(drop: Drop): void {
        if (drop.step >= 0 && drop.step < FALL_STEPS) {
            drop.sprite.draw(16 + Math.floor(drop.step / 6), drop.x, drop.y);
        } else if (drop.step >= FALL_STEPS && drop.step < SPLASH_END) {
            drop.sprite.draw(drop.step, drop.x, drop.y);
        } else {
            drop.sprite.sprite.setVisible(false);
        }
    }
}
