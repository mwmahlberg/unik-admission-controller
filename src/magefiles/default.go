//go:build mage

package main

import "github.com/magefile/mage/sh"

// Build builds the binary
func Build() {
	sh.RunV("go", "build", ".")
}
