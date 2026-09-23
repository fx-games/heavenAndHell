package rules

// Object kinds: rows of xl_obj (objdat.cpp AllObjects, _object_id).
const (
	ObjL1LIGHT      = 0
	ObjL1LDOOR      = 1
	ObjL1RDOOR      = 2
	ObjSKFIRE       = 3
	ObjLEVER        = 4
	ObjCHEST1       = 5
	ObjCHEST2       = 6
	ObjCHEST3       = 7
	ObjCANDLE1      = 8
	ObjCANDLE2      = 9
	ObjCANDLEO      = 10
	ObjBANNERL      = 11
	ObjBANNERM      = 12
	ObjBANNERR      = 13
	ObjSKPILE       = 14
	ObjSKSTICK1     = 15
	ObjSKSTICK2     = 16
	ObjSKSTICK3     = 17
	ObjSKSTICK4     = 18
	ObjSKSTICK5     = 19
	ObjCRUX1        = 20
	ObjCRUX2        = 21
	ObjCRUX3        = 22
	ObjSTAND        = 23
	ObjANGEL        = 24
	ObjBOOK2L       = 25
	ObjBCROSS       = 26
	ObjNUDEW2R      = 27
	ObjSWITCHSKL    = 28
	ObjTNUDEM1      = 29
	ObjTNUDEM2      = 30
	ObjTNUDEM3      = 31
	ObjTNUDEM4      = 32
	ObjTNUDEW1      = 33
	ObjTNUDEW2      = 34
	ObjTNUDEW3      = 35
	ObjTORTURE1     = 36
	ObjTORTURE2     = 37
	ObjTORTURE3     = 38
	ObjTORTURE4     = 39
	ObjTORTURE5     = 40
	ObjBOOK2R       = 41
	ObjL2LDOOR      = 42
	ObjL2RDOOR      = 43
	ObjTORCHL       = 44
	ObjTORCHR       = 45
	ObjTORCHL2      = 46
	ObjTORCHR2      = 47
	ObjSARC         = 48
	ObjFLAMEHOLE    = 49
	ObjFLAMELVR     = 50
	ObjWATER        = 51
	ObjBOOKLVR      = 52
	ObjTRAPL        = 53
	ObjTRAPR        = 54
	ObjBOOKSHELF    = 55
	ObjWEAPRACK     = 56
	ObjBARREL       = 57
	ObjBARRELEX     = 58
	ObjSHRINEL      = 59
	ObjSHRINER      = 60
	ObjSKELBOOK     = 61
	ObjBOOKCASEL    = 62
	ObjBOOKCASER    = 63
	ObjBOOKSTAND    = 64
	ObjBOOKCANDLE   = 65
	ObjBLOODFTN     = 66
	ObjDECAP        = 67
	ObjTCHEST1      = 68
	ObjTCHEST2      = 69
	ObjTCHEST3      = 70
	ObjBLINDBOOK    = 71
	ObjBLOODBOOK    = 72
	ObjPEDISTAL     = 73
	ObjL3LDOOR      = 74
	ObjL3RDOOR      = 75
	ObjPURIFYINGFTN = 76
	ObjARMORSTAND   = 77
	ObjARMORSTANDN  = 78
	ObjGOATSHRINE   = 79
	ObjCAULDRON     = 80
	ObjMURKYFTN     = 81
	ObjTEARFTN      = 82
	ObjALTBOY       = 83
	ObjMCIRCLE1     = 84
	ObjMCIRCLE2     = 85
	ObjSTORYBOOK    = 86
	ObjSTORYCANDLE  = 87
	ObjSTEELTOME    = 88
	ObjWARARMOR     = 89
	ObjWARWEAP      = 90
	ObjTBCROSS      = 91
	ObjWEAPONRACK   = 92
	ObjWEAPONRACKN  = 93
	ObjMUSHPATCH    = 94
	ObjLAZSTAND     = 95
	ObjSLAINHERO    = 96
	ObjSIGNCHEST    = 97
)

// Monster types (monster_id) the placement code names.
const (
	MtWSKELAX  = 8
	MtXSKELAX  = 11
	MtWSKELBW  = 20
	MtXSKELBW  = 23
	MtWSKELSD  = 24
	MtXSKELSD  = 27
	MtNGOATMC  = 34
	MtGGOATMC  = 37
	MtNGOATBW  = 42
	MtGGOATBW  = 45
	MtSKING    = 50
	MtCLEAVER  = 51
	MtGOLEM    = 109
	MtDIABLO   = 110
	MtADVOCATE = 108
	MtRBLACK   = 96
	MtCOUNSLR  = 105
)

// D1's own Nightmare bonuses (InitMonster, Monster::level/exp/toHit and the gold roll in devilutionX). This game
// has no difficulty selection; it applies them once Diablo has been beaten, so a second run still has teeth.
const (
	NightmareLevelBonus     = 15
	NightmareAcBonus        = 50
	NightmareToHitBonus     = 85
	NightmareItemLevelBonus = 16
)

// Level types (dungeon_type).
const (
	DTypeTown = iota
	DTypeCathedral
	DTypeCatacombs
	DTypeCaves
	DTypeHell
)
