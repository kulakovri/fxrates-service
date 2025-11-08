package domain

type Pair string

var SupportedPairs = map[string]bool{
	"USD/EUR": true,
	"EUR/USD": true,
	"USD/MXN": true,
	"MXN/USD": true,
	"EUR/MXN": true,
	"MXN/EUR": true,
}

func ValidatePair(p string) bool { return SupportedPairs[p] }
