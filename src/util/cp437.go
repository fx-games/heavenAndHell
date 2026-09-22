package util

// Package cp437 provides conversions between IBM code page 437 and UTF-8.
var cp437 = []rune("\x00☺☻♥♦♣♠•◘○◙♂♀♪♬☼►◄↕‼¶§▬↨↑↓→←∟↔▲▼ !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~⌂ÇüéâäàåçêëèïîìÄÅÉæÆôöòûùÿÖÜ¢£¥₧ƒáíóúñÑªº¿⌐¬½¼¡«»░▒▓│┤╡╢╖╕╣║╗╝╜╛┐└┴┬├─┼╞╟╚╔╩╦╠═╬╧╨╤╥╙╘╒╓╫╪┘┌█▄▌▐▀αßΓπΣσµτΦΘΩδ∞φε∩≡±≥≤⌠⌡÷≈°∙·√ⁿ²■\u00A0")

var reverse = func(r []rune) map[rune]byte {
	m := make(map[rune]byte)
	for i, c := range r {
		m[c] = byte(i)
	}
	return m
}(cp437)

// Byte takes a Unicode code point and returns the corresponding CP437 byte,
// or '?' if there is no corresponding CP437 byte.
func UnicodeToCP437Byte(r rune) byte {
	if b, ok := reverse[r]; ok {
		return b
	}
	return '?'
}
