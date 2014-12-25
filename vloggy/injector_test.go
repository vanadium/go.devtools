package main

import (
	"go/token"
	"path"
	"strconv"
	"testing"
)

const (
	failingPrefix       = "failschecks"
	failingPackageCount = 7
	testPackagePrefix   = "v.io/tools/vloggy/testdata"
)

func TestValidPackages(t *testing.T) {
	pkg := path.Join(testPackagePrefix, "passeschecks")
	_, methods := doTest(t, []string{pkg})
	if len(methods) > 0 {
		t.Fatalf("Test package %q failed to pass the log checks", pkg)
	}
}

func TestInvalidPackages(t *testing.T) {
	for i := 1; i <= failingPackageCount; i++ {
		pkg := path.Join(testPackagePrefix, failingPrefix, "test"+strconv.Itoa(i))
		_, methods := doTest(t, []string{pkg})
		if len(methods) == 0 {
			t.Fatalf("Test package %q passes log checks but it should not", pkg)
		}
	}
}

func doTest(t *testing.T, packages []string) (*token.FileSet, map[funcDeclRef]error) {
	interfaceList := []string{path.Join(testPackagePrefix, "iface")}

	prog, err := load(interfaceList, packages, []string{"testpackage"})
	if err != nil {
		t.Fatal(err)
	}

	interfacePackages, implementationPackages := findPackages(prog, interfaceList, packages)

	interfaces := findPublicInterfaces(interfacePackages)
	if len(interfaces) == 0 {
		t.Fatalf("Log injector did not find any interfaces in %v", interfacePackages)
	}

	methods := findMethodsImplementing(implementationPackages, interfaces)
	if len(methods) == 0 {
		t.Fatalf("Log injector could not find any methods implementing the test interfaces in %v", implementationPackages)
	}

	return prog.Fset, checkMethods(methods)
}
