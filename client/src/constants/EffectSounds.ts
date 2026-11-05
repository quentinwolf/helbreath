/**
 * Sounds played by spell effects (game/spells). Values are Phaser audio cache keys;
 * GameScene loads them on entering the game (see EFFECT_SOUND_FILES).
 * Numbers are the original client's 'E' sound IDs (PlaySound('E', n, ...)).
 */
const fx = (n: number) => `fx-E${n}`;

export const ENERGY_BOLT_LAUNCH_SOUND = fx(1);
export const ENERGY_STRIKE_SOUND = fx(1);
export const EFFECT_ENERGY_EXPLOSION = fx(2);
export const MAGIC_MISSILE_EXPLOSION_SOUND = fx(3);
export const EFFECT_EXPLOSION = fx(4);
export const EFFECT_CAST = fx(5);
export const ITEM_DROPPED_SOUND = fx(12);
export const LIGHTNING_SOUND = fx(40);
export const CHILL_WIND_DROPLET_SOUND = fx(45);
export const EFFECT_ICE_SHARD = fx(46);
export const EFFECT_SNOW_IMPACT = fx(47);

/** Cache key -> file name under assets/sounds. */
export const EFFECT_SOUND_FILES: Record<string, string> = Object.fromEntries(
    [1, 2, 3, 4, 5, 12, 40, 45, 46, 47].map((n) => [fx(n), `E${n}.mp3`]),
);
