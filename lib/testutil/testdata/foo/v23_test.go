// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY
package foo_test

import "testing"
import "os"

import "v.io/x/ref/lib/testutil"
import "v.io/x/ref/lib/testutil/v23tests"

func TestMain(m *testing.M) {
	testutil.Init()
	// TODO(cnicolaou): call modules.Dispatch and remove the need for TestHelperProcess
	os.Exit(m.Run())
}

func TestV23(t *testing.T) {
	v23tests.RunTest(t, V23Test)
}
