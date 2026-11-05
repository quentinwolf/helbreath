import type { Scene } from 'phaser';

type UpdateFn = (time: number, delta: number) => void;

// Per-frame callbacks registered by moving spell effects. Phaser keeps scene.events
// listeners across a scene restart (GameScene restarts on every teleport), so an
// in-flight projectile would keep ticking against destroyed sprites. GameScene calls
// clearSpellUpdates() on shutdown to drop them all.
const active = new Map<Scene, Set<UpdateFn>>();

export function onSceneUpdate(scene: Scene, fn: UpdateFn): void {
    let set = active.get(scene);
    if (!set) {
        set = new Set();
        active.set(scene, set);
    }
    set.add(fn);
    scene.events.on('update', fn);
}

export function offSceneUpdate(scene: Scene, fn: UpdateFn): void {
    active.get(scene)?.delete(fn);
    scene.events.off('update', fn);
}

export function clearSpellUpdates(scene: Scene): void {
    const set = active.get(scene);
    if (!set) return;
    for (const fn of set) scene.events.off('update', fn);
    set.clear();
}
