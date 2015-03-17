package one_test

import (
	"fmt"
	"io"
	"os"
	"testing"

	"v.io/x/ref/test/modules"
	"v.io/x/ref/test/v23tests"
)

func V23TestOneA(i *v23tests.T) {}

func V23TestOneB(i *v23tests.T) {}

func modulesOneExt(stdin io.Reader, stdout io.Writer, stderr io.Writer, env map[string]string, args ...string) error {
	fmt.Fprintln(stdout, "modulesOneExt")
	return nil
}

func modulesTwoExt(stdin io.Reader, stdout io.Writer, stderr io.Writer, env map[string]string, args ...string) error {
	fmt.Fprintln(stdout, "modulesTwoExt")
	return nil
}

func TestModulesOneExt(t *testing.T) {
	sh, err := modules.NewShell(nil, nil, false, t)
	if err != nil {
		t.Fatal(err)
	}
	for _, cmd := range []string{"modulesOneExt", "modulesTwoExt"} {
		m, err := sh.Start(cmd, nil)
		if err != nil {
			if m != nil {
				m.Shutdown(os.Stderr, os.Stderr)
			}
			t.Fatal(err)
		}
		m.Expect(cmd)
	}
}
