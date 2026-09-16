package d1

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

// objTypeConv maps .DUN object numbers to object kinds (0 = none).
var objTypeConv = []int{0, 4, 20, 21, 22, 24, 11, 12, 13, 0, 0, 0, 0, 0, 25, 41, 26, 0, 8, 9, 10, 80, 0, 0, 0, 0, 0, 0, 0, 0, 49, 0, 0, 0, 0, 0, 84, 85, 3, 14, 15, 16, 17, 18, 19, 0, 0, 0, 0, 0, 0, 28, 0, 53, 54, 36, 37, 38, 39, 40, 0, 0, 0, 0, 0, 27, 0, 0, 0, 0, 29, 30, 31, 32, 33, 34, 35, 5, 5, 5, 6, 6, 6, 7, 7, 7, 0, 0, 0, 0, 0, 73, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 83, 0, 0, 89, 90, 47, 46, 94}

// monstConvTbl maps .DUN monster numbers (minus 1) to monster types.
var monstConvTbl = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 29, 30, 31, 32, 34, 35, 36, 37, 38, 40, 39, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 52, 53, 54, 55, 56, 57, 59, 58, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 0, 0, 0, 0, 72, 73, 74, 75, 0, 0, 0, 0, 77, 76, 78, 79, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 92, 91, 93, 94, 95, 96, 97, 98, 99, 100, 101, 102, 103, 104, 105, 106, 107, 108, 0, 110, 0, 109, 0, 0, 0, 0, 0, 0, 0, 0, 0, 80, 111}

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
	// Hellfire's
	MtHORKSPWN = 117
	MtHORKDMN  = 123
	MtDEFILER  = 124
	MtARCHLICH = 133
	MtNAKRUL   = 137
)
