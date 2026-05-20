package assets

import "embed"

// FS contains version-pinned Camoufox Python reference assets.
//
//go:embed data/*
var FS embed.FS

func ReadFile(name string) ([]byte, error) {
	return FS.ReadFile("data/" + name)
}
