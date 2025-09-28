//go:build mage

package main

import (
	"os"
	"os/exec"
	"runtime"
)

var binDir = "bin"

func Build() error {
	os.MkdirAll(binDir, 0755)
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}
	if err := sh("go", "build", "-o", binDir+"/clipd"+ext, "./cmd/clipd"); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := sh("go", "build", "-o", binDir+"/server"+ext, "./cmd/server"); err != nil {
			return err
		}
	}
	return nil
}

func Clean() error {
	return os.RemoveAll(binDir)
}

func Format() error {
	return sh("go", "fmt", "./...")
}

func sh(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
