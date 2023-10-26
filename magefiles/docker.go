//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

type Docker mg.Namespace

// Build builds the docker image
func (d Docker) Build() {
	sh.RunV("docker", "build", "-t", "ghcr.io/mwmahlberg/unik-admission-controller:latest", ".")
}

func (d Docker) Push() {
	sh.RunV("docker", "push", "ghcr.io/mwmahlberg/unik-admission-controller:latest")
}
