// NPC type ID -> sprite file name (IDs match the original NPC.cfg). Used by the game
// (GameScene) and the map editor.
export const NPC_SPRITE_MAP: Record<number, string> = {
  // Monsters
  10: 'slm',        // Slime
  11: 'ske',        // Skeleton
  12: 'gol',        // Stone-Golem
  13: 'cyc',        // Cyclops
  14: 'orc',        // Orc
  16: 'ant',        // Giant-Ant
  17: 'scp',        // Scorpion
  18: 'zom',        // Zombie
  22: 'amp',        // Amphis (snake)
  23: 'cla',        // Clay-Golem
  27: 'helb',       // Hellbound
  28: 'troll',      // Troll
  29: 'orge',       // Ogre
  30: 'liche',      // Liche
  31: 'demon',      // Demon
  32: 'unicorn',    // Unicorn
  33: 'werewolf',   // WereWolf
  48: 'stalker',    // Stalker
  49: 'hellclaw',   // Hellclaw
  50: 'tigerworm',  // Tigerworm
  52: 'gagoyle',    // Gagoyle
  53: 'beholder',   // Beholder
  54: 'darkelf',    // Dark-Elf
  55: 'bunny',      // Rabbit
  56: 'cat',        // Cat
  57: 'giantfrog',  // Frog
  58: 'mtgiant',    // Mountain-Giant
  59: 'ettin',      // Ettin
  60: 'giantplant', // Plant
  61: 'rudolph',    // Rudolph
  62: 'direboar',   // DireBoar
  63: 'frost',      // Frost
  65: 'icegolem',   // Ice-Golem
  66: 'wyvern',     // Wyvern
  71: 'centaurus',  // Centaurus
  72: 'clawturtle', // Claw-Turtle
  73: 'firewyvern', // Fire-Wyvern
  74: 'giantcrayfish', // Giant-Crayfish
  75: 'giantlizard',  // Giant-Lizard
  76: 'giantplant',   // Giant-Tree
  77: 'mastermageorc', // MasterMage-Orc
  78: 'minotaurs',  // Minotaurs
  79: 'nizie',      // Nizie
  80: 'tentocle',   // Tentocle
  // Town NPCs
  15: 'shopkpr',    // ShopKeeper-W
  19: 'gandlf',     // Gandlf
  20: 'howard',     // Howard
  21: 'guard',      // Guard
  24: 'tom',        // Tom
  25: 'william',    // William
  26: 'kennedy',    // Kennedy
  34: 'dummy',      // Dummy
  64: 'crop',       // Crops
  67: 'mcgaffin',   // McGaffin
  68: 'perry',      // Perry
  69: 'devlin',     // Devlin
  90: 'gail',       // Gail
  // Crusade / Heldenian war units and bosses (original client's MakeSprite table)
  36: 'gt-arrow',      // Arrow Guard Tower
  37: 'gt-cannon',     // Cannon Guard Tower
  38: 'manacollector', // Mana Collector
  39: 'detector',      // Detector
  40: 'esg',           // ESG
  41: 'gmg',           // GMG
  42: 'manastone',     // Mana Stone
  43: 'lwb',           // Light War Beetle
  44: 'ghk',           // God's Hand Knight
  45: 'ghkabs',        // God's Hand Knight on Armored Battle Steed
  46: 'tk',            // Temple Knight
  47: 'bg',            // Battle Golem
  51: 'catapult',      // Catapult
  70: 'barlog',        // Barlog ("Dragon" in NPC.cfg uses this sprite)
  81: 'yspro',         // Abaddon
  82: 'sorceress',     // Sorceress
  83: 'tpknight',      // TP Knight
  84: 'elfmaster',     // Elf Master
  85: 'darkknight',    // Dark Knight
  86: 'hbtank',        // Heavy Battle Tank
  87: 'cbturret',      // CB Turret
  88: 'babarian',      // Barbarian
  89: 'acannon',       // A-Cannon
  97: 'fireelemental', // Fire Elemental (FireEl)
};
