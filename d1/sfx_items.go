package d1

// ItemSnds is the sound an item makes landing on the floor (ItemDropSnds) and going into the inventory or being
// used (ItemInvSnds), by its cursor graphic (ItemCAnimTbl). Generated from devilution 1.09's items.cpp, HELLFIRE.
func ItemSnds(curs int) (drop, inv string) {
	if curs < 0 || curs >= len(itemCAnimTbl) {
		return "", ""
	}
	a := itemCAnimTbl[curs]
	return itemDropSnds[a], itemInvSnds[a]
}

var itemCAnimTbl = [...]int{
	20, 16, 16, 16, 4, 4, 4, 12, 12, 12,
	12, 12, 12, 12, 12, 21, 21, 25, 12, 28,
	28, 28, 38, 38, 38, 32, 38, 38, 38, 24,
	24, 26, 2, 25, 22, 23, 24, 25, 27, 27,
	29, 0, 0, 0, 12, 12, 12, 12, 12, 0,
	8, 8, 0, 8, 8, 8, 8, 8, 8, 6,
	8, 8, 8, 6, 8, 8, 6, 8, 8, 6,
	6, 6, 8, 8, 8, 5, 9, 13, 13, 13,
	5, 5, 5, 15, 5, 5, 18, 18, 18, 30,
	5, 5, 14, 5, 14, 13, 16, 18, 5, 5,
	7, 1, 3, 17, 1, 15, 10, 14, 3, 11,
	8, 0, 1, 7, 0, 7, 15, 7, 3, 3,
	3, 6, 6, 11, 11, 11, 31, 14, 14, 14,
	6, 6, 7, 3, 8, 14, 0, 14, 14, 0,
	33, 1, 1, 1, 1, 1, 7, 7, 7, 14,
	14, 17, 17, 17, 0, 34, 1, 0, 3, 17,
	8, 8, 6, 1, 3, 3, 11, 3, 12, 12,
	12, 12, 12, 12, 12, 12, 12, 12, 12, 12,
	12, 12, 12, 12, 12, 12, 12, 35, 39, 36,
	36, 36, 37, 38, 38, 38, 38, 38, 41, 42,
	8, 8, 8, 17, 0, 6, 8, 11, 11, 3,
	3, 1, 6, 6, 6, 1, 8, 6, 11, 3,
	6, 8, 1, 6, 6, 17, 40, 0, 0,
}

var itemDropSnds = [...]string{
	"IS_FHARM", "IS_FAXE", "IS_FPOT", "IS_FBOW", "IS_GOLD", "IS_FCAP",
	"IS_FSWOR", "IS_FSHLD", "IS_FSWOR", "IS_FROCK", "IS_FAXE", "IS_FSTAF",
	"IS_FRING", "IS_FCAP", "IS_FLARM", "IS_FSHLD", "IS_FSCRL", "IS_FHARM",
	"IS_FBOOK", "IS_FLARM", "IS_FPOT", "IS_FPOT", "IS_FPOT", "IS_FPOT",
	"IS_FPOT", "IS_FPOT", "IS_FPOT", "IS_FPOT", "IS_FBODY", "IS_FBODY",
	"IS_FMUSH", "IS_ISIGN", "IS_FBLST", "IS_FANVL", "IS_FSTAF", "IS_FROCK",
	"IS_FSCRL", "IS_FSCRL", "IS_FROCK", "IS_FMUSH", "IS_FHARM", "IS_FLARM",
	"IS_FLARM",
}

var itemInvSnds = [...]string{
	"IS_IHARM", "IS_IAXE", "IS_IPOT", "IS_IBOW", "IS_GOLD", "IS_ICAP",
	"IS_ISWORD", "IS_ISHIEL", "IS_ISWORD", "IS_IROCK", "IS_IAXE", "IS_ISTAF",
	"IS_IRING", "IS_ICAP", "IS_ILARM", "IS_ISHIEL", "IS_ISCROL", "IS_IHARM",
	"IS_IBOOK", "IS_IHARM", "IS_IPOT", "IS_IPOT", "IS_IPOT", "IS_IPOT",
	"IS_IPOT", "IS_IPOT", "IS_IPOT", "IS_IPOT", "IS_IBODY", "IS_IBODY",
	"IS_IMUSH", "IS_ISIGN", "IS_IBLST", "IS_IANVL", "IS_ISTAF", "IS_IROCK",
	"IS_ISCROL", "IS_ISCROL", "IS_IROCK", "IS_IMUSH", "IS_IHARM", "IS_ILARM",
	"IS_ILARM",
}
