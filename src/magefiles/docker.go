/* 
 *     docker.go is part of github.com/unik-k8s/admission-controller.
 *  
 *     Copyright 2023 Markus W Mahlberg <07.federkleid-nagelhaut@icloud.com>
 *  
 *     Licensed under the Apache License, Version 2.0 (the "License");
 *     you may not use this file except in compliance with the License.
 *     You may obtain a copy of the License at
 *  
 *         http://www.apache.org/licenses/LICENSE-2.0
 *  
 *     Unless required by applicable law or agreed to in writing, software
 *     distributed under the License is distributed on an "AS IS" BASIS,
 *     WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *     See the License for the specific language governing permissions and
 *     limitations under the License.
 *  
 */

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
	mg.Deps(Docker.Build)
	sh.RunV("docker", "push", "ghcr.io/mwmahlberg/unik-admission-controller:latest")
}
