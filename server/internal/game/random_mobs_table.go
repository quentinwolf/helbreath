package game

// The random mob generator's tables, ported from HGServer 2.24b (Game.cpp
// MobGenerator) with their structure kept, so they can be checked line by line
// against the original. Codes are the original's monster numbers
// (randomMobNames); the level is a map's random-mob-generator level.
// Ported by a one-off script; edit by hand from here on.

// randomMobCode rolls the monster a map of this level spawns. firmBerserk: the
// original spawns it berserk (level 21's Unicorns; not ported).
func randomMobCode(level int, iDice func(n, s int) int) (iResult int, firmBerserk bool) {
	iResult = iDice(1, 100)
	// Some levels set a group size here; the original overwrites it straight
	// after (randomMobFollowers), so it has no effect.
	var iTotalMob int
	switch level {
	case 1:
		if (iResult >= 1) && (iResult < 20) {
			iResult = 1 // Slime
		} else if (iResult >= 20) && (iResult < 40) {
			iResult = 2 // Giant-Ant
		} else if (iResult >= 40) && (iResult < 85) {
			iResult = 24 // Rabbit
		} else if (iResult >= 85) && (iResult < 95) {
			iResult = 25 // Cat
		} else if (iResult >= 95) && (iResult <= 100) {
			iResult = 3 // Orc
		}
	case 2:
		if (iResult >= 1) && (iResult < 40) {
			iResult = 1
		} else if (iResult >= 40) && (iResult < 80) {
			iResult = 2
		} else {
			iResult = 10
		}
	case 3:
		if (iResult >= 1) && (iResult < 20) {
			switch iDice(1, 2) {
			case 1:
				iResult = 3
			case 2:
				iResult = 4
			}
		} else if (iResult >= 20) && (iResult < 25) {
			iResult = 30
		} else if (iResult >= 25) && (iResult < 50) {
			switch iDice(1, 3) {
			case 1:
				iResult = 5
			case 2:
				iResult = 6
			case 3:
				iResult = 7
			}
		} else if (iResult >= 50) && (iResult < 75) {
			switch iDice(1, 7) {
			case 1, 2:
				iResult = 8
			case 3:
				iResult = 11
			case 4:
				iResult = 12
			case 5:
				iResult = 18
			case 6:
				iResult = 26
			case 7:
				iResult = 28
			}
		} else if (iResult >= 75) && (iResult <= 100) {
			switch iDice(1, 5) {
			case 1, 2:
				iResult = 9
			case 3:
				iResult = 13
			case 4:
				iResult = 14
			case 5:
				iResult = 27
			}
		}
	case 4:
		if (iResult >= 1) && (iResult < 50) {
			switch iDice(1, 2) {
			case 1:
				iResult = 2
			case 2:
				iResult = 10
			}
		} else if (iResult >= 50) && (iResult < 80) {
			switch iDice(1, 2) {
			case 1:
				iResult = 8
			case 2:
				iResult = 11
			}
		} else if (iResult >= 80) && (iResult <= 100) {
			switch iDice(1, 2) {
			case 1:
				iResult = 14
			case 2:
				iResult = 9
			}
		}
	case 5:
		if (iResult >= 1) && (iResult < 30) {
			switch iDice(1, 5) {
			case 1, 2, 3, 4, 5:
				iResult = 2
			}
		} else if (iResult >= 30) && (iResult < 60) {
			switch iDice(1, 2) {
			case 1:
				iResult = 3
			case 2:
				iResult = 4
			}
		} else if (iResult >= 60) && (iResult < 80) {
			switch iDice(1, 2) {
			case 1:
				iResult = 5
			case 2:
				iResult = 7
			}
		} else if (iResult >= 80) && (iResult < 95) {
			switch iDice(1, 3) {
			case 1, 2:
				iResult = 8
			case 3:
				iResult = 11
			}
		} else if (iResult >= 95) && (iResult <= 100) {
			switch iDice(1, 3) {
			case 1:
				iResult = 11
			case 2:
				iResult = 14
			case 3:
				iResult = 9
			}
		}
	case 6:
		if (iResult >= 1) && (iResult < 60) {
			switch iDice(1, 4) {
			case 1:
				iResult = 5 // Skeleton
			case 2:
				iResult = 6 // Orc-Mage
			case 3:
				iResult = 12 // Cyclops
			case 4:
				iResult = 11 // Troll
			}
		} else if (iResult >= 60) && (iResult < 90) {
			switch iDice(1, 5) {
			case 1, 2:
				iResult = 8 // Stone-Golem
			case 3:
				iResult = 11 // Troll
			case 4:
				iResult = 12 // Cyclops
			case 5:
				iResult = 43 // Tentocle
			}
		} else if (iResult >= 90) && (iResult <= 100) {
			switch iDice(1, 9) {
			case 1:
				iResult = 26
			case 2:
				iResult = 9
			case 3:
				iResult = 13
			case 4:
				iResult = 14
			case 5:
				iResult = 18
			case 6:
				iResult = 28
			case 7:
				iResult = 27
			case 8:
				iResult = 29
			}
		}
	case 7:
		if (iResult >= 1) && (iResult < 50) {
			switch iDice(1, 5) {
			case 1:
				iResult = 3 // Orc
			case 2:
				iResult = 6 // Orc-Mage
			case 3:
				iResult = 10 // Amphis
			case 4:
				iResult = 3 // Orc
			case 5:
				iResult = 50 // Giant-Tree
			}
		} else if (iResult >= 50) && (iResult < 85) {
			switch iDice(1, 4) {
			case 1:
				iResult = 50 // Giant-Tree
			case 2, 3:
				iResult = 6 // Orc-Mage
			case 4:
				iResult = 12 // Troll
			}
		} else if (iResult >= 85) && (iResult <= 100) {
			switch iDice(1, 4) {
			case 1:
				iResult = 12 // Troll
			case 2, 3:
				if iDice(1, 100) < 3 {
					iResult = 17 // Unicorn
				} else {
					iResult = 12
				} // Troll
			case 4:
				iResult = 29 // Cannibal-Plant
			}
		}
	case 8:
		if (iResult >= 1) && (iResult < 70) {
			switch iDice(1, 2) {
			case 1:
				iResult = 4
			case 2:
				iResult = 5
			}
		} else if (iResult >= 70) && (iResult < 95) {
			switch iDice(1, 2) {
			case 1:
				iResult = 8
			case 2:
				iResult = 11
			}
		} else if (iResult >= 95) && (iResult <= 100) {
			iResult = 14
		}
	case 9:
		if (iResult >= 1) && (iResult < 70) {
			switch iDice(1, 2) {
			case 1:
				iResult = 4
			case 2:
				iResult = 5
			}
		} else if (iResult >= 70) && (iResult < 95) {
			switch iDice(1, 3) {
			case 1:
				iResult = 8
			case 2:
				iResult = 9
			case 3:
				iResult = 13
			}
		} else if (iResult >= 95) && (iResult <= 100) {
			switch iDice(1, 6) {
			case 1, 2, 3:
				iResult = 9
			case 4, 5:
				iResult = 14
			case 6:
				iResult = 15
			}
		}
		if (iDice(1, 3) == 1) && (iResult != 16) {
			firmBerserk = true
		}
	case 10:
		if (iResult >= 1) && (iResult < 70) {
			switch iDice(1, 3) {
			case 1:
				iResult = 9
			case 2:
				iResult = 5
			case 3:
				iResult = 8
			}
		} else if (iResult >= 70) && (iResult < 95) {
			switch iDice(1, 3) {
			case 1, 2:
				iResult = 13
			case 3:
				iResult = 14
			}
		} else if (iResult >= 95) && (iResult <= 100) {
			switch iDice(1, 3) {
			case 1, 2:
				iResult = 14
			case 3:
				iResult = 15
			}
		}
		if (iDice(1, 3) == 1) && (iResult != 16) {
			firmBerserk = true
		}
	case 11:
		if (iResult >= 1) && (iResult < 30) {
			switch iDice(1, 5) {
			case 1, 2, 3, 4, 5:
				iResult = 2
			}
		} else if (iResult >= 30) && (iResult < 60) {
			switch iDice(1, 2) {
			case 1:
				iResult = 3
			case 2:
				iResult = 4
			}
		} else if (iResult >= 60) && (iResult < 80) {
			switch iDice(1, 2) {
			case 1:
				iResult = 5
			case 2:
				iResult = 7
			}
		} else if (iResult >= 80) && (iResult < 95) {
			switch iDice(1, 3) {
			case 1, 2:
				iResult = 10
			case 3:
				iResult = 11
			}
		} else if (iResult >= 95) && (iResult <= 100) {
			switch iDice(1, 3) {
			case 1:
				iResult = 11
			case 2:
				iResult = 7
			case 3:
				iResult = 8
			}
		}
	case 12:
		if (iResult >= 1) && (iResult < 50) {
			switch iDice(1, 3) {
			case 1:
				iResult = 1
			case 2:
				iResult = 2
			case 3:
				iResult = 10
			}
		} else if (iResult >= 50) && (iResult < 85) {
			switch iDice(1, 2) {
			case 1:
				iResult = 5
			case 2:
				iResult = 4
			}
		} else if (iResult >= 85) && (iResult <= 100) {
			switch iDice(1, 3) {
			case 1:
				iResult = 8
			case 2:
				iResult = 11
			case 3:
				iResult = 26
			}
		}
	case 13:
		if (iResult >= 1) && (iResult < 15) {
			iResult = 4
			firmBerserk = true
			iTotalMob = 4 - (iDice(1, 2) - 1)
		} else if (iResult >= 15) && (iResult < 40) {
			iResult = 14
			firmBerserk = true
			iTotalMob = 4 - (iDice(1, 2) - 1)
		} else if (iResult >= 40) && (iResult < 60) {
			iResult = 9
			firmBerserk = true
			iTotalMob = 4 - (iDice(1, 2) - 1)
		} else if (iResult >= 60) && (iResult < 75) {
			iResult = 13
			firmBerserk = true
			iTotalMob = 4 - (iDice(1, 2) - 1)
		} else if (iResult >= 75) && (iResult < 95) {
			iResult = 23
		} else if (iResult >= 95) && (iResult <= 100) {
			iResult = 22
		}
	case 14:
		if (iResult >= 1) && (iResult < 30) {
			iResult = 23 // Dark-Elf
		} else if (iResult >= 30) && (iResult < 50) {
			iResult = 31 // Ice-Golem
		} else if (iResult >= 50) && (iResult < 70) {
			iResult = 22 // Beholder
			firmBerserk = true
			iTotalMob = 4 - (iDice(1, 2) - 1)
		} else if (iResult >= 70) && (iResult < 90) {
			iResult = 32 // DireBoar
		} else if (iResult >= 90) && (iResult <= 100) {
			iResult = 33 // Frost
		}
	case 15:
		if (iResult >= 1) && (iResult < 35) {
			iResult = 23
			firmBerserk = true
		} else if (iResult >= 35) && (iResult < 50) {
			iResult = 22
			firmBerserk = true
		} else if (iResult >= 50) && (iResult < 80) {
			iResult = 15
		} else if (iResult >= 80) && (iResult <= 100) {
			iResult = 21
		}
	case 16:
		if (iResult >= 1) && (iResult < 40) {
			switch iDice(1, 3) {
			case 1:
				iResult = 7 // Scorpion
			case 2:
				iResult = 2 // Giant-Ant
			case 3:
				iResult = 10 // Amphis
			}
		} else if (iResult >= 40) && (iResult < 50) {
			iResult = 30 // Rudolph
		} else if (iResult >= 50) && (iResult < 85) {
			switch iDice(1, 2) {
			case 1:
				iResult = 5 // Skeleton
			case 2:
				iResult = 4 // Zombie
			}
		} else if (iResult >= 85) && (iResult <= 100) {
			switch iDice(1, 3) {
			case 1:
				iResult = 8 // Stone-Golem
			case 2:
				iResult = 11 // Clay-Golem
			case 3:
				iResult = 7 // Scorpion
			}
		}
	case 17:
		if (iResult >= 1) && (iResult < 30) {
			switch iDice(1, 4) {
			case 1:
				iResult = 22 // Giant-Frog
			case 2:
				iResult = 8 // Stone-Golem
			case 3:
				iResult = 24 // Rabbit
			case 4:
				iResult = 5
			}
		} else if (iResult >= 30) && (iResult < 40) {
			iResult = 30
		} else if (iResult >= 40) && (iResult < 70) {
			iResult = 32
		} else if (iResult >= 70) && (iResult < 90) {
			iResult = 31
			if iDice(1, 5) == 1 {
				firmBerserk = true
			}
		} else if (iResult >= 90) && (iResult <= 100) {
			iResult = 33
		}
	case 18:
		if (iResult >= 1) && (iResult < 2) {
			iResult = 39 // Tentocle
		} else if (iResult >= 2) && (iResult < 12) {
			iResult = 44 // ClawTurtle
		} else if (iResult >= 12) && (iResult < 50) {
			iResult = 48 // Nizie
		} else if (iResult >= 50) && (iResult < 80) {
			iResult = 45 // Giant-Crayfish
		} else if (iResult >= 80) && (iResult < 90) {
			iResult = 34 // Stalker
		} else if (iResult >= 90) && (iResult <= 100) {
			iResult = 26 // Giant-Frog
		}
	case 19:
		if (iResult >= 1) && (iResult < 15) {
			iResult = 44
		} else if (iResult >= 15) && (iResult < 25) {
			iResult = 46
		} else if (iResult >= 25) && (iResult < 35) {
			iResult = 21
		} else if (iResult >= 35) && (iResult < 60) {
			iResult = 43
		} else if (iResult >= 60) && (iResult < 85) {
			iResult = 23
		} else if (iResult >= 85) && (iResult <= 100) {
			iResult = 22
		}
	case 20:
		if (iResult >= 1) && (iResult < 2) {
			iResult = 41
		} else if (iResult >= 2) && (iResult < 3) {
			iResult = 40
		} else if (iResult >= 3) && (iResult < 8) {
			iResult = 53
		} else if (iResult >= 8) && (iResult < 9) {
			iResult = 39
		} else if (iResult >= 9) && (iResult < 20) {
			iResult = 21
		} else if (iResult >= 20) && (iResult < 35) {
			iResult = 16
		} else if (iResult >= 35) && (iResult < 45) {
			iResult = 44
		} else if (iResult >= 45) && (iResult < 55) {
			iResult = 45
		} else if (iResult >= 55) && (iResult < 75) {
			iResult = 28
		} else if (iResult >= 75) && (iResult < 95) {
			iResult = 43
		} else if (iResult >= 95) && (iResult < 100) {
			iResult = 22
		}
	case 21:
		if (iResult >= 1) && (iResult < 94) {
			iResult = 17 // Unicorn
			firmBerserk = true
		} else if (iResult >= 94) && (iResult < 95) {
			iResult = 36 // Wyvern
		} else if (iResult >= 95) && (iResult < 96) {
			iResult = 37 // Fire-Wyvern
		} else if (iResult >= 96) && (iResult < 97) {
			iResult = 47 // MasterMage-Orc
		} else if (iResult >= 97) && (iResult < 98) {
			iResult = 35 // Hellclaw
		} else if (iResult >= 98) && (iResult < 99) {
			iResult = 49 // Tigerworm
		} else if (iResult >= 99) && (iResult <= 100) {
			iResult = 51 // Abaddon
		}
	}
	_ = iTotalMob
	return iResult, firmBerserk
}

// randomMobFollowers is how many of the same monster come with it.
func randomMobFollowers(iResult int, iDice func(n, s int) int) (iTotalMob int) {
	switch iResult {
	case 1:
		iTotalMob = iDice(1, 5) - 1
	case 2:
		iTotalMob = iDice(1, 5) - 1
	case 3:
		iTotalMob = iDice(1, 5) - 1
	case 4:
		iTotalMob = iDice(1, 3) - 1
	case 5:
		iTotalMob = iDice(1, 3) - 1
	case 6:
		iTotalMob = iDice(1, 3) - 1
	case 7:
		iTotalMob = iDice(1, 3) - 1
	case 8:
		iTotalMob = iDice(1, 2) - 1
	case 9:
		iTotalMob = iDice(1, 2) - 1
	case 10:
		iTotalMob = iDice(1, 5) - 1
	case 11:
		iTotalMob = iDice(1, 3) - 1
	case 12:
		iTotalMob = iDice(1, 5) - 1
	case 13:
		iTotalMob = iDice(1, 3) - 1
	case 14:
		iTotalMob = iDice(1, 2) - 1
	case 15:
		iTotalMob = iDice(1, 3) - 1
	case 16:
		iTotalMob = iDice(1, 2) - 1
	case 17:
		iTotalMob = iDice(1, 2) - 1
	case 18:
		iTotalMob = iDice(1, 5) - 1
	case 19:
		iTotalMob = iDice(1, 2) - 1
	case 20:
		iTotalMob = iDice(1, 2) - 1
	case 21:
		iTotalMob = iDice(1, 5) - 1
	case 22:
		iTotalMob = iDice(1, 2) - 1
	case 23:
		iTotalMob = iDice(1, 2) - 1
	case 24:
		iTotalMob = iDice(1, 4) - 1
	case 25:
		iTotalMob = iDice(1, 2) - 1
	case 26:
		iTotalMob = iDice(1, 3) - 1
	case 27:
		iTotalMob = iDice(1, 3) - 1
	case 28:
		iTotalMob = iDice(1, 3) - 1
	case 29:
		iTotalMob = iDice(1, 5) - 1
	case 30:
		iTotalMob = iDice(1, 3) - 1
	case 31:
		iTotalMob = iDice(1, 3) - 1
	case 32:
		iTotalMob = 1
	case 33:
		iTotalMob = 1
	case 34:
		iTotalMob = 1
	case 35:
		iTotalMob = 1
	case 36:
		iTotalMob = 1
	case 37:
		iTotalMob = 1
	case 38:
		iTotalMob = 1
	case 39:
		iTotalMob = 1
	case 40:
		iTotalMob = 1
	case 41:
		iTotalMob = 1
	case 42:
		iTotalMob = iDice(1, 3) - 1
	case 43:
		iTotalMob = 1
	case 44:
		iTotalMob = iDice(1, 3) - 1
	case 45:
		iTotalMob = 1
	default:
		iTotalMob = 0
	}
	if iTotalMob > 2 {
		switch iResult {
		case 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 32:
			if iDice(1, 5) == 1 {
				iTotalMob = 0
			} // 75% ???????? ???.
		case 33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 44, 45, 46, 47, 48, 49:
			if iDice(1, 5) != 1 {
				iTotalMob = 0
			} // 75% ???????????????
		}
	}
	return iTotalMob
}

// randomMobNames are the original's monster names by code (any other code is
// an Orc, as there). Several aren't monsters this port has; those don't spawn.
var randomMobNames = map[int]string{
	1:  "Slime",
	2:  "Giant-Ant",
	3:  "Orc",
	4:  "Zombie",
	5:  "Skeleton",
	6:  "Orc-Mage",
	7:  "Scorpion",
	8:  "Stone-Golem",
	9:  "Cyclops",
	10: "Amphis",
	11: "Clay-Golem",
	12: "Troll",
	13: "Orge",
	14: "Hellbound",
	15: "Liche",
	16: "Demon",
	17: "Unicorn",
	18: "WereWolf",
	19: "YB-Aresden",
	20: "YB-Elvine",
	21: "Gagoyle",
	22: "Beholder",
	23: "Dark-Elf",
	24: "Rabbit",
	25: "Cat",
	26: "Giant-Frog",
	27: "Mountain-Giant",
	28: "Ettin",
	29: "Cannibal-Plant",
	30: "Rudolph",
	31: "Ice-Golem",
	32: "DireBoar",
	33: "Frost",
	34: "Stalker",
	35: "Hellclaw",
	36: "Wyvern",
	37: "Fire-Wyvern",
	38: "Barlog",
	39: "Tentocle",
	40: "Centaurus",
	41: "Giant-Lizard",
	42: "Minotaurs",
	43: "Tentocle",
	44: "Claw-Turtle",
	45: "Giant-Crayfish",
	46: "Giant-Plant",
	47: "MasterMage-Orc",
	48: "Nizie",
	49: "Tigerworm",
	50: "Giant-Plant",
	51: "Abaddon",
	52: "YW-Aresden",
	53: "YW-Elvine",
	54: "YY-Aresden",
	55: "YY-Elvine",
	56: "XB-Aresden",
	57: "XB-Elvine",
	58: "XW-Aresden",
	59: "XW-Elvine",
	60: "XY-Aresden",
	61: "XY-Elvine",
}
