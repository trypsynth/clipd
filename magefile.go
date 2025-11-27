//go:build mage

package main

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/magefile/mage/sh"
)

var binDir = "bin"

func Build() error {
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return err
	}
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	if err := sh.RunV("go", "build", "-o", filepath.Join(binDir, "clipd"+ext), "./cmd/clipd"); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := sh.RunV("go", "build", "-ldflags", "-H windowsgui", "-o", filepath.Join(binDir, "server"+ext), "./cmd/server"); err != nil {
			return err
		}
	}
	return nil
}

func Clean() error {
	return os.RemoveAll(binDir)
}

func Fmt() error {
	return sh.RunV("go", "fmt", "./...")
}

func Vet() error {
	return sh.RunV("go", "vet", "./...")
}
