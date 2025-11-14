package domain

import "regexp"

type Pair string

var SupportedCurrency = map[string]bool{
	"USD": true,
	"EUR": true,
	"MXN": true,
}

var pairRe = regexp.MustCompile(`^[A-Z]{3}/[A-Z]{3}$`)

func ValidatePair(p string) bool {
	// First validate format via shared precompiled regex
	if !pairRe.MatchString(p) {
		return false
	}
	// Then validate supported currencies and disallow identical base/quote
	base := p[:3]
	quote := p[4:]
	return SupportedCurrency[base] && SupportedCurrency[quote] && base != quote
}
