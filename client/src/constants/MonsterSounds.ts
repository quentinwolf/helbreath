// Monster sounds per NPC sprite: move, attack, hurt and death. Sound names are
// the original files in assets/sounds (e.g. M1 = M1.mp3).
// Mapping from ErkoKnoll/helbreath-base-game (MIT), constants/Monsters.ts.

export type MonsterSoundSet = { move?: string; attack?: string; hurt?: string; death?: string };

export const MONSTER_SOUNDS: Record<string, MonsterSoundSet> = {
    amp: { move: 'M25', attack: 'M26', hurt: 'M27', death: 'M28' },
    ant: { move: 'M29', attack: 'M30', hurt: 'M31', death: 'M32' },
    beholder: { move: 'E46', attack: 'C6', hurt: 'M3', death: 'C7' },
    bunny: { move: 'M71', attack: 'M75', hurt: 'M79', death: 'M83' },
    cat: { move: 'M72', attack: 'M76', hurt: 'M80', death: 'M84' },
    centaurus: { move: 'M117', attack: 'M119', hurt: 'M116', death: 'C7' },
    cla: { move: 'M37', attack: 'M38', hurt: 'M39', death: 'M40' },
    clawturtle: { move: 'M114', attack: 'M115', hurt: 'M112', death: 'M113' },
    cyc: { move: 'M41', attack: 'M42', hurt: 'M43', death: 'M44' },
    darkelf: { move: 'C8', attack: 'C3', hurt: 'C13', death: 'M150' },
    demon: { move: 'M59', attack: 'M61', hurt: 'M60', death: 'M62' },
    direboar: { move: 'M87', attack: 'M78', hurt: 'M78', death: 'C7' },
    ettin: { move: 'M91', attack: 'M92', hurt: 'M93', death: 'M94' },
    firewyvern: { move: 'M106', attack: 'M107', hurt: 'M105', death: 'M105' },
    frost: { move: 'M23', attack: 'C4', hurt: 'C13', death: 'C7' },
    gagoyle: { move: 'M37', attack: 'C2', hurt: 'M43', death: 'C7' },
    giantcrayfish: { move: 'M99', attack: 'M100', hurt: 'M101', death: 'M98' },
    giantfrog: { move: 'M73', attack: 'M77', hurt: 'M81', death: 'M85' },
    giantlizard: { move: 'M126', attack: 'M127', hurt: 'M124', death: 'M125' },
    giantplant: { move: 'M122', attack: 'M123', hurt: 'M120', death: 'M121' },
    gol: { move: 'M33', attack: 'M34', hurt: 'M35', death: 'M36' },
    guard: { move: 'C8', attack: 'C2', hurt: 'C12', death: 'C7' },
    helb: { move: 'M5', attack: 'M6', hurt: 'M7', death: 'M8' },
    hellclaw: { move: 'M41', attack: 'M42', hurt: 'M43', death: 'M44' },
    icegolem: { move: 'M33', attack: 'M34', hurt: 'M35', death: 'M36' },
    liche: { move: 'M55', attack: 'M56', hurt: 'M57', death: 'M58' },
    mastermageorc: { move: 'M74', attack: 'M78', hurt: 'M86', death: 'M86' },
    minotaurs: { move: 'M46', attack: 'M123', hurt: 'M102', death: 'M103' },
    mtgiant: { move: 'M87', attack: 'M88', hurt: 'M89', death: 'M90' },
    nizie: { move: 'M134', attack: 'M135', hurt: 'M132', death: 'M133' },
    orc: { move: 'M9', attack: 'M10', hurt: 'M11', death: 'M12' },
    orge: { move: 'M51', attack: 'M52', hurt: 'M53', death: 'M54' },
    rudolph: { move: 'C11', attack: 'M38', hurt: 'M59', death: 'M65' },
    scp: { move: 'M21', attack: 'M22', hurt: 'M23', death: 'M24' },
    ske: { move: 'M13', attack: 'M14', hurt: 'M15', death: 'M16' },
    slm: { move: 'M1', attack: 'M2', hurt: 'M3', death: 'M4' },
    stalker: { move: 'M9', attack: 'M10', hurt: 'M11', death: 'M12' },
    tentocle: { move: 'M110', attack: 'M111', hurt: 'M108', death: 'M109' },
    tigerworm: { move: 'M1', attack: 'C1', death: 'M58' },
    troll: { move: 'M46', attack: 'M47', hurt: 'M48', death: 'M49' },
    unicorn: { move: 'M63', attack: 'M64', hurt: 'M65', death: 'M66' },
    werewolf: { move: 'M67', attack: 'M68', hurt: 'M69', death: 'M70' },
    wyvern: { move: 'M106', attack: 'M107', hurt: 'M105', death: 'M105' },
    zom: { move: 'M17', attack: 'M18', hurt: 'M19', death: 'M20' },
};

/** Phaser audio cache key for an original sound file name. */
export function monsterSoundKey(name: string): string {
    return `snd-${name}`;
}
