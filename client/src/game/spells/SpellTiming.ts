// When a damage spell's effect visibly hits, in ms after the cast. The server
// applies spell damage at once (as the original did); the client holds the
// hits, deaths and loot of that cast back until this moment, so a monster dies
// when the meteor lands, not when the spell is clicked. Timings follow the
// effect settings in SpellEffects.ts.

const TILE = 32;

export function spellImpactMs(spellId: number, fromX: number, fromY: number, toX: number, toY: number): number {
    const dist = Math.hypot((toX - fromX) * TILE, (toY - fromY) * TILE); // pixels
    let ms = 0;
    switch (spellId) {
        case 0: case 37: case 56: // Magic Missile, Lightning Arrow, Mass Lightning Arrow: 50 px per 20 ms
            ms = dist / 2.5; break;
        case 10: case 47: case 20: case 30: case 61: case 6: // bolts, fire balls / strikes: 1500 px/s
            ms = dist / 1.5; break;
        case 60: // Energy Strike: 2000 px/s
            ms = dist / 2.0; break;
        case 81: // Meteor Strike: falls from 300 px right / 600 px up at 1500 px/s
            ms = 450; break;
        case 57: case 72: // Ice Strike, Mass Ice Strike: shards fade in for 7 frames of 20 ms, then drop
            ms = 350; break;
        case 45: case 63: // Chill Wind, Mass Chill Wind: 200 px drop at 1500 px/s
            ms = 130; break;
        case 85: case 91: // Blizzard, Mass Blizzard: 300 ms out, first shards land 200 ms later
            ms = 500; break;
        case 70: case 96: // Bloody Shock Wave, Earth Shock Wave: 400 ms waves
            ms = 400; break;
    }
    return Math.min(ms, 1500);
}
