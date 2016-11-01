// +build linux

/*
http://www.apache.org/licenses/LICENSE-2.0.txt
Copyright 2016 IBM Corporation
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/


//
// This snap plugin exposes two metrics:
//
// 1. /ibm/sysfs/iocore/nr_iocores
// Returns the number of active I/O cores
//
// 2. /ibm/sysfs/iocore/*/cpu_utilization
// Returns the effective CPU utilization (cpu1, cpu2, cpu (aggregated))
//


package main

import (
	"os"

	// Import the snap plugin library
	"github.com/intelsdi-x/snap/control/plugin"

	// Import our collector plugin implementation
	"github.com/intelsdi-x/snap-plugin-collector-iocore/iocore"
)

func main() {

	p, err := iocore.New()
	if err != nil {
		panic(err)
	}

	plugin.Start(
		iocore.Meta(),
		p,
		os.Args[1],
	)
}
