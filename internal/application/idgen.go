package application

type IDGen interface{ New() string }

type defaultIDGen struct{}

func (defaultIDGen) New() string { return "" }
