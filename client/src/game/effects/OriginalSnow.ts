import type { Scene } from 'phaser';
import { PivotSprite } from '../spells/OriginalFx';

// Snow ported from the original client (Client.cpp WhetherObjectFrameCounter /
// DrawWhetherEffects, weather types 4-6). Each flake is effect2.spr sheet 1: it
// lives 80 steps of 30 ms, falling (80 - step) / 10 px a step (8 px at first,
// slowing to a stop) and drifting -1..+1 px sideways each step. It is drawn with
// frames 39 + (step / 20) * 3 + a random 0-2, so it shrinks through four sizes
// and twinkles between three shapes, then respawns after a short random delay.
// The original also piled snow on its bottom HUD bar; our HUD is HTML, so that
// part isn't ported.

export const SNOW_SPRITE = 'effect2';
const SHEET = 1;
const STEP_MS = 30;
const LIFE_STEPS = 80;
const FIRST_FRAME = 39;
const DEPTH = 999990;

interface Flake {
    x: number;
    y: number;
    step: number;
    variant: number;
    sprite: PivotSprite;
}

export class OriginalSnow {
    private readonly scene: Scene;
    private readonly flakes: Flake[] = [];
    private elapsed = 0;

    constructor(scene: Scene, count: number) {
        this.scene = scene;
        for (let i = 0; i < count; i++) {
            const sprite = new PivotSprite(scene, SNOW_SPRITE, SHEET);
            sprite.sprite.setDepth(DEPTH);
            const flake: Flake = { x: 0, y: 0, step: 0, variant: 0, sprite };
            // Original SetWhetherStatus: each flake starts up to 40 steps late
            this.respawn(flake, -Math.floor(Math.random() * 40));
            this.flakes.push(flake);
        }
    }

    update(delta: number): void {
        this.elapsed += delta;
        // Catch up after a slow frame, but not after a long pause (tab hidden)
        const steps = Math.min(Math.floor(this.elapsed / STEP_MS), 5);
        this.elapsed = steps === 5 ? 0 : this.elapsed - steps * STEP_MS;
        for (let s = 0; s < steps; s++) this.tick();
        for (const flake of this.flakes) this.draw(flake);
    }

    destroy(): void {
        for (const flake of this.flakes) flake.sprite.destroy();
        this.flakes.length = 0;
    }

    private tick(): void {
        for (const flake of this.flakes) {
            flake.step++;
            if (flake.step >= 0 && flake.step < LIFE_STEPS) {
                flake.y += Math.floor((LIFE_STEPS - flake.step) / 10);
                flake.x += 1 - Math.floor(Math.random() * 3);
                flake.variant = Math.floor(Math.random() * 3);
            } else if (flake.step >= LIFE_STEPS) {
                this.respawn(flake, -Math.floor(Math.random() * 10));
            }
        }
    }

    /**
     * The original spawned flakes from 92 px left of the view to 848 px right of
     * it (800 px wide screen) and from 256 px above it to 544 px down it; a flake
     * falls about 290 px in its life, so snow covers the whole screen. Same
     * spread here, sized to our camera.
     */
    private respawn(flake: Flake, step: number): void {
        const cam = this.scene.cameras.main;
        flake.x = cam.scrollX - 100 + Math.random() * (cam.width + 140);
        flake.y = cam.scrollY - 256 + Math.random() * (cam.height + 200);
        flake.step = step;
    }

    private draw(flake: Flake): void {
        if (flake.step >= 0 && flake.step < LIFE_STEPS) {
            const frame = FIRST_FRAME + Math.floor(flake.step / 20) * 3 + flake.variant;
            flake.sprite.draw(frame, flake.x, flake.y);
        } else {
            flake.sprite.sprite.setVisible(false);
        }
    }
}
