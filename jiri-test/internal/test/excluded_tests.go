// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// A simple utility to display tests that are to be excluded on the
// host that this command is run on. It also displays the go
// environment variables and USER values in effect.
//
// You can run it as you would any other go main program that's
// contained in a single file within a related package:
//
// 1) if you obtained the code using 'go get':
// "go run $(go list -f {{.Dir}} v.io/x/devtools/v23/internal/test)/excluded_tests.go"
//
// 2) if you are using the jiri tool and "JIRI_ROOT" setup.
// "jiri go run $(jiri go list -f {{.Dir}} v.io/x/devtools/v23/internal/test)/excluded_tests.go"
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"v.io/x/devtools/v23/internal/test"
)

var integrationFlag = flag.Bool("v23.tests", false, "Additionally display the tests excluded only when running integration tests.")
var raceFlag = flag.Bool("race", false, "Additionally display the tests excluded only when running under the go race detector.")

func main() {
	flag.Parse()
	fmt.Printf("GOOS: %s\n", runtime.GOOS)
	fmt.Printf("GOARCH: %s\n", runtime.GOARCH)
	fmt.Printf("GOROOT: %s\n", runtime.GOROOT())
	fmt.Printf("USER: %q\n", os.Getenv("USER"))

	fmt.Println("Excluded tests:")
	excluded := test.ExcludedTests()
	for _, t := range excluded {
		fmt.Printf("%#v\n", t)
	}

	if *raceFlag {
		fmt.Println("Excluded race tests:")
		raceExcluded := test.ExcludedRaceTests()
		for _, t := range raceExcluded {
			fmt.Printf("%#v\n", t)
		}
	}

	if *integrationFlag {
		fmt.Println("Excluded integration tests:")
		integrationExcluded := test.ExcludedIntegrationTests()
		for _, t := range integrationExcluded {
			fmt.Printf("%#v\n", t)
		}
	}
}
