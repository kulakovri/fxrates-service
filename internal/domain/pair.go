package domain

import "unicode"

type Pair string

var SupportedCurrency = map[string]bool{
	"USD": true,
	"EUR": true,
	"MXN": true,
}

func IsSupportedPair(p Pair) bool {
	base, quote, ok := SplitPair(string(p))
	if !ok {
		return false
	}
	return SupportedCurrency[base] && SupportedCurrency[quote] && base != quote
}

func SplitPair(p string) (string, string, bool) {
	if len(p) != 7 || p[3] != '/' {
		return "", "", false
	}
	base := p[:3]
	quote := p[4:]
	if !isAlpha(base) || !isAlpha(quote) {
		return "", "", false
	}
	return base, quote, true
}

func isAlpha(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

func ValidatePair(p string) bool { return IsSupportedPair(Pair(p)) }
