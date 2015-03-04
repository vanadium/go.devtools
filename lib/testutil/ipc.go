package testutil

import (
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"v.io/x/devtools/lib/collect"
	"v.io/x/devtools/lib/util"
	"v.io/x/devtools/lib/xunit"
)

const (
	testNumServerNodes      = 5
	testNumClientNodes      = 10
	testNumWorkersPerClient = 15
	testMaxChunkCnt         = 100
	testMaxPayloadSize      = 10000
	testDuration            = 1 * time.Hour
	testServerUpTime        = testDuration + 10*time.Minute
	testWaitTimeForServerUp = 3 * time.Minute
	testPort                = 10000

	gceProject           = "vanadium-internal"
	gceZone              = "asia-east1-a"
	gceServerMachineType = "n1-highcpu-8"
	gceClientMachineType = "n1-highcpu-4"
	gceNodePrefix        = "tmpnode-ipc-stress"

	vcloudPkg = "v.io/x/devtools/vcloud"
	serverPkg = "v.io/x/ref/profiles/internal/ipc/stress/stressd"
	clientPkg = "v.io/x/ref/profiles/internal/ipc/stress/stress"
)

var (
	binPath = filepath.Join("release", "go", "bin")
)

// vanadiumGoIPCStress runs an IPC stress test with multiple GCE instances.
func vanadiumGoIPCStress(ctx *util.Context, testName string, _ ...TestOpt) (_ *TestResult, e error) {
	cleanup, err := initTest(ctx, testName, []string{})
	if err != nil {
		return nil, internalTestError{err, "Init"}
	}
	defer collect.Error(func() error { return cleanup() }, &e)

	// Install binaries.
	if err := ctx.Run().Command("v23", "go", "install", vcloudPkg, serverPkg, clientPkg); err != nil {
		return nil, internalTestError{err, "Install Binaries"}
	}

	// Cleanup old nodes if any.
	if err := deleteNodes(ctx); err != nil {
		fmt.Fprintf(ctx.Stdout(), "IGNORED: %v\n", err)
	}

	// Create nodes.
	if err := createNodes(ctx); err != nil {
		return nil, internalTestError{err, "Create Nodes"}
	}

	// Start servers.
	serverDone, err := startServers(ctx)
	if err != nil {
		return nil, internalTestError{err, "Run Servers"}
	}

	// Run the test.
	result, err := runTest(ctx, testName)
	if err != nil {
		return nil, internalTestError{err, "Run Test"}
	}

	// Wait for servers to stop.
	if err := <-serverDone; err != nil {
		return nil, internalTestError{err, "Stop Servers"}
	}

	// Delete nodes.
	if err := deleteNodes(ctx); err != nil {
		return nil, internalTestError{err, "Delete Nodes"}
	}

	return result, nil
}

func serverNodeName(n int) string {
	return fmt.Sprintf("%s-server-%02d", gceNodePrefix, n)
}

func clientNodeName(n int) string {
	return fmt.Sprintf("%s-client-%02d", gceNodePrefix, n)
}

func createNodes(ctx *util.Context) error {
	root, err := util.VanadiumRoot()
	if err != nil {
		return err
	}

	cmd := filepath.Join(root, binPath, "vcloud")
	args := []string{
		"node", "create",
		"-project", gceProject,
		"-zone", gceZone,
	}

	serverArgs := append(args, "-machine_type", gceServerMachineType)
	for n := 0; n < testNumServerNodes; n++ {
		serverArgs = append(serverArgs, serverNodeName(n))
	}
	if err := ctx.Run().Command(cmd, serverArgs...); err != nil {
		return err
	}

	clientArgs := append(args, "-machine_type", gceClientMachineType)
	for n := 0; n < testNumClientNodes; n++ {
		clientArgs = append(clientArgs, clientNodeName(n))
	}
	return ctx.Run().Command(cmd, clientArgs...)
}

func deleteNodes(ctx *util.Context) error {
	root, err := util.VanadiumRoot()
	if err != nil {
		return err
	}

	cmd := filepath.Join(root, binPath, "vcloud")
	args := []string{
		"node", "delete",
		"-project", gceProject,
		"-zone", gceZone,
	}
	for n := 0; n < testNumServerNodes; n++ {
		args = append(args, serverNodeName(n))
	}
	for n := 0; n < testNumClientNodes; n++ {
		args = append(args, clientNodeName(n))
	}

	return ctx.Run().Command(cmd, args...)
}

func startServers(ctx *util.Context) (<-chan error, error) {
	root, err := util.VanadiumRoot()
	if err != nil {
		return nil, err
	}

	var servers []string
	for n := 0; n < testNumServerNodes; n++ {
		servers = append(servers, serverNodeName(n))
	}

	cmd := filepath.Join(root, binPath, "vcloud")
	args := []string{
		"run",
		"-failfast",
		"-project", gceProject,
		strings.Join(servers, ","),
		filepath.Join(root, binPath, "stressd"),
		"++",
		"./stressd",
		"-veyron.tcp.address", fmt.Sprintf(":%d", testPort),
		"-duration", testServerUpTime.String(),
	}

	done := make(chan error)
	go func() {
		done <- ctx.Run().Command(cmd, args...)
	}()

	// Wait until for a few minute while servers are brought up.
	timeout := time.After(testWaitTimeForServerUp)
	select {
	case err := <-done:
		if err != nil {
			return nil, err
		}
		close(done)
	case <-timeout:
	}
	return done, nil
}

func runTest(ctx *util.Context, testName string) (*TestResult, error) {
	root, err := util.VanadiumRoot()
	if err != nil {
		return nil, err
	}

	var servers, clients []string
	for n := 0; n < testNumServerNodes; n++ {
		servers = append(servers, fmt.Sprintf("/%s:%d", serverNodeName(n), testPort))
	}
	for n := 0; n < testNumClientNodes; n++ {
		clients = append(clients, clientNodeName(n))
	}

	var out bytes.Buffer
	opts := ctx.Run().Opts()
	opts.Stdout = io.MultiWriter(opts.Stdout, &out)
	opts.Stderr = io.MultiWriter(opts.Stderr, &out)
	cmd := filepath.Join(root, binPath, "vcloud")
	args := []string{
		"run",
		"-failfast",
		"-project", gceProject,
		strings.Join(clients, ","),
		filepath.Join(root, binPath, "stress"),
		"++",
		"./stress",
		"-servers", strings.Join(servers, ","),
		"-workers", strconv.Itoa(testNumWorkersPerClient),
		"-max_chunk_count", strconv.Itoa(testMaxChunkCnt),
		"-max_payload_size", strconv.Itoa(testMaxPayloadSize),
		"-duration", testDuration.String(),
	}
	if err = ctx.Run().CommandWithOpts(opts, cmd, args...); err != nil {
		return nil, err
	}

	// Get the ipc stats from the servers and stop them.
	args = []string{
		"run",
		"-failfast",
		"-project", gceProject,
		clients[0],
		filepath.Join(root, binPath, "stress"),
		"++",
		"./stress",
		"-servers", strings.Join(servers, ","),
		"-workers", "0",
		"-duration", "0",
		"-server_stats",
		"-server_stop",
	}
	if err = ctx.Run().CommandWithOpts(opts, cmd, args...); err != nil {
		return nil, err
	}

	// Verify the ipc stats.
	cStats, sStats, err := readStats(out.String())
	if err != nil {
		suite := xunit.CreateTestSuiteWithFailure("StressTest", "ReadStats", "Failure", err.Error(), 0)
		if err := xunit.CreateReport(ctx, testName, []xunit.TestSuite{*suite}); err != nil {
			return nil, err
		}
		return &TestResult{Status: TestFailed}, nil
	}

	fmt.Fprint(ctx.Stdout(), "\nRESULT:\n")
	fmt.Fprintf(ctx.Stdout(), "client ipc stats: %+v\n", *cStats)
	fmt.Fprintf(ctx.Stdout(), "server ipc stats: %+v\n", *sStats)
	fmt.Fprint(ctx.Stdout(), "\n")

	if cStats.sumCount != sStats.sumCount || cStats.sumStreamCount != sStats.sumStreamCount {
		output := fmt.Sprintf("%v != %v", cStats, sStats)
		suite := xunit.CreateTestSuiteWithFailure("StressTest", "VerifyStats", "Mismatched", output, 0)
		if err := xunit.CreateReport(ctx, testName, []xunit.TestSuite{*suite}); err != nil {
			return nil, err
		}
		return &TestResult{Status: TestFailed}, nil
	}

	return &TestResult{Status: TestPassed}, nil
}

type stressStats struct {
	sumCount       uint64
	sumStreamCount uint64
}

func readStats(out string) (*stressStats, *stressStats, error) {
	re := regexp.MustCompile(`client stats: {SumCount:(\d+) SumStreamCount:(\d+)}`)
	n, cStats, err := readOneStats(re, out)
	if err != nil {
		return nil, nil, err
	}
	if n != testNumClientNodes {
		return nil, nil, fmt.Errorf("invalid number of client stats: %d", n)
	}

	re = regexp.MustCompile(`server stats: ".+":{SumCount:(\d+) SumStreamCount:(\d+)}`)
	n, sStats, err := readOneStats(re, out)
	if err != nil {
		return nil, nil, err
	}
	if n != testNumServerNodes {
		return nil, nil, fmt.Errorf("invalid number of server stats: %d", n)
	}

	return cStats, sStats, nil
}

func readOneStats(re *regexp.Regexp, out string) (int, *stressStats, error) {
	var stats stressStats
	matches := re.FindAllStringSubmatch(out, -1)
	for _, match := range matches {
		if len(match) != 3 {
			return 0, nil, fmt.Errorf("invalid stats: %v", match)
		}
		sumCount, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return 0, nil, fmt.Errorf("%v: %v", err, match)
		}
		sumStreamCount, err := strconv.ParseUint(match[2], 10, 64)
		if err != nil {
			return 0, nil, fmt.Errorf("%v: %v", err, match)
		}
		if sumCount == 0 || sumStreamCount == 0 {
			// Although clients choose servers and IPC methods randomly, we report
			// this as a failure since it is very unlikely.
			return 0, nil, fmt.Errorf("zero count: %v", match)
		}
		stats.sumCount += sumCount
		stats.sumStreamCount += sumStreamCount
	}
	return len(matches), &stats, nil
}
