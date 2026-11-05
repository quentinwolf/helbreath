import type { Scene } from 'phaser';
import { TILE_SIZE } from '../Config';

/** What spell effects need from the camera: a shake on impact. */
export interface CameraManager {
    setCameraShake(effectPixelX: number, effectPixelY: number, multiplier?: number): void;
}

/**
 * The original client's impact shake (SetCameraShakingEffect): strength falls off with
 * the impact's distance from the screen centre, then decays by one pixel per frame.
 * GameScene positions the camera every frame, then calls apply() to add the jitter.
 */
export class CameraShake implements CameraManager {
    private degree = 0;

    constructor(private scene: Scene) {}

    public setCameraShake(effectPixelX: number, effectPixelY: number, multiplier?: number): void {
        const camera = this.scene.cameras?.main;
        if (!camera) return;

        const camCenterX = camera.scrollX + camera.width / 2;
        const camCenterY = camera.scrollY + camera.height / 2;
        const distTiles = Math.max(Math.abs(effectPixelX - camCenterX), Math.abs(effectPixelY - camCenterY)) / TILE_SIZE;

        let degree = Math.max(0, (5 - distTiles) * 2);
        if (multiplier) degree *= multiplier;
        if (degree <= 2) return;
        this.degree = Math.max(this.degree, degree);
    }

    /** Call once per frame after the camera has been positioned. */
    public apply(): void {
        if (this.degree <= 0) return;
        const camera = this.scene.cameras.main;
        const d = Math.round(this.degree);
        camera.scrollX += d - Phaser.Math.Between(0, d * 2 - 1);
        camera.scrollY += d - Phaser.Math.Between(0, d * 2 - 1);
        this.degree = Math.max(0, this.degree - 1);
    }

    public reset(): void {
        this.degree = 0;
    }
}
