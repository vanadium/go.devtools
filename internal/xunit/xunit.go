// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package xunit contains types and functions for manipulating xunit
// files.
package xunit

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"v.io/x/devtools/internal/tool"
	"v.io/x/devtools/internal/util"
)

type TestSuites struct {
	Suites  []TestSuite `xml:"testsuite"`
	XMLName xml.Name    `xml:"testsuites"`
}

type TestSuite struct {
	Name     string     `xml:"name,attr"`
	Cases    []TestCase `xml:"testcase"`
	Errors   int        `xml:"errors,attr"`
	Failures int        `xml:"failures,attr"`
	Skip     int        `xml:"skip,attr"`
	Tests    int        `xml:"tests,attr"`
}

type TestCase struct {
	Name      string    `xml:"name,attr"`
	Classname string    `xml:"classname,attr"`
	Errors    []Error   `xml:"error"`
	Failures  []Failure `xml:"failure"`
	Time      string    `xml:"time,attr"`
	Skipped   []string  `xml:"skipped"`
}

type Error struct {
	Message string `xml:"message,attr"`
	Data    string `xml:",chardata"`
}

type Failure struct {
	Message string `xml:"message,attr"`
	Data    string `xml:",chardata"`
}

// CreateReport generates an xUnit report using the given test suites.
func CreateReport(ctx *tool.Context, testName string, suites []TestSuite) error {
	result := TestSuites{Suites: suites}
	bytes, err := xml.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("MarshalIndent(%v) failed: %v", result, err)
	}
	if err := ctx.Run().WriteFile(ReportPath(testName), bytes, os.FileMode(0644)); err != nil {
		return fmt.Errorf("WriteFile(%v) failed: %v", ReportPath(testName), err)
	}
	return nil
}

// CreateTestSuiteWithFailure encodes the given information as a test
// suite with a single failure.
func CreateTestSuiteWithFailure(pkgName, testName, failureMessage, failureOutput string, duration time.Duration) *TestSuite {
	s := TestSuite{Name: pkgName}
	c := TestCase{
		Classname: pkgName,
		Name:      testName,
		Time:      fmt.Sprintf("%.2f", duration.Seconds()),
	}
	s.Tests = 1
	f := Failure{
		Message: failureMessage,
		Data:    failureOutput,
	}
	c.Failures = append(c.Failures, f)
	s.Failures = 1
	s.Cases = append(s.Cases, c)
	return &s
}

// CreateFailureReport creates an xUnit report for the given failure.
func CreateFailureReport(ctx *tool.Context, testName, pkgName, testCaseName, failureMessage, failureOutput string) error {
	s := CreateTestSuiteWithFailure(pkgName, testCaseName, failureMessage, failureOutput, 0)
	if err := CreateReport(ctx, testName, []TestSuite{*s}); err != nil {
		return err
	}
	return nil
}

// ReportPath returns the path to the xUnit file.
//
// TODO(jsimsa): Once all Jenkins shell test scripts are ported to Go,
// change the filename to xunit_report_<testName>.xml.
func ReportPath(testName string) string {
	workspace, fileName := os.Getenv("WORKSPACE"), fmt.Sprintf("tests_%s.xml", strings.Replace(testName, "-", "_", -1))
	if workspace == "" {
		return filepath.Join(os.Getenv("HOME"), "tmp", testName, fileName)
	} else {
		return filepath.Join(workspace, fileName)
	}
}

// TestSuiteFromGoTestOutput reads data from the given input, assuming
// it contains test results generated by "go test -v", and returns it
// as an in-memory data structure.
func TestSuiteFromGoTestOutput(ctx *tool.Context, testOutput io.Reader) (*TestSuite, error) {
	root, err := util.VanadiumRoot()
	if err != nil {
		return nil, err
	}
	bin, err := util.ThirdPartyBinPath(root, "go2xunit")
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	opts := ctx.Run().Opts()
	opts.Stdin = testOutput
	opts.Stdout = &out
	if err := ctx.Run().CommandWithOpts(opts, bin); err != nil {
		return nil, err
	}
	var suite TestSuite
	if err := xml.Unmarshal(out.Bytes(), &suite); err != nil {
		return nil, fmt.Errorf("Unmarshal() failed: %v\n%v", err, out.String())
	}
	return &suite, nil
}
