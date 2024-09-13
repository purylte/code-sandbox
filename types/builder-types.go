package types

type Code struct {
	Text     string
	Language Language
}

type Language int

const (
	Go Language = iota
	CPP
)
