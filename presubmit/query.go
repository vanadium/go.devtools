// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"

	"v.io/jiri"
	"v.io/jiri/collect"
	"v.io/jiri/gerrit"
	"v.io/jiri/project"
	"v.io/jiri/tool"
	"v.io/x/devtools/tooldata"
	"v.io/x/lib/cmdline"
)

const (
	defaultLogFilePath = "${HOME}/tmp/presubmit_log"
)

var (
	queryStringFlag string
	logFilePathFlag string

	emailWhitelist = []string{
		"aaron@azinman.com",
		"aaron@empiric.al",
	}
)

func init() {
	cmdQuery.Flags.StringVar(&queryStringFlag, "query", defaultQueryString, "The string used to query Gerrit for open CLs.")
	cmdQuery.Flags.StringVar(&logFilePathFlag, "log-file", os.ExpandEnv(defaultLogFilePath), "The file that stores the refs from the previous Gerrit query.")
	cmdQuery.Flags.Lookup("log-file").DefValue = defaultLogFilePath

	tool.InitializeProjectFlags(&cmdQuery.Flags)
}

// clNumberToPatchsetMap is a map from CL numbers to the latest patchset of the CL.
type clNumberToPatchsetMap map[int]int

// cmdQuery represents the 'query' command of the presubmit tool.
var cmdQuery = &cmdline.Command{
	Name:  "query",
	Short: "Query open CLs from Gerrit",
	Long: `
This subcommand queries open CLs from Gerrit, calculates diffs from the previous
query results, and sends each one with related metadata (ref, project, changeId)
to a Jenkins job which will run tests against the corresponding CL and post
review with test results.
`,
	Runner: jiri.RunnerFunc(runQuery),
}

// runQuery implements the "query" subcommand.
func runQuery(jirix *jiri.X, args []string) error {
	numSentCLs := 0
	defer func() {
		printf(jirix.Stdout(), "%d sent.\n", numSentCLs)
	}()

	jenkinsObj, err := jirix.Jenkins(jenkinsHostFlag)
	if err != nil {
		return err
	}

	// Don't query anything if the last "presubmit-test" build failed.
	lastBuildInfo, err := jenkinsObj.LastCompletedBuildStatus(presubmitTestJobFlag, nil)
	if err != nil {
		fmt.Fprintf(jirix.Stderr(), "%v\n", err)
	} else {
		if lastBuildInfo.Result == "FAILURE" {
			printf(jirix.Stdout(), "%s is failing. Skipping this round.\n", presubmitTestJobFlag)
			return nil
		}
	}

	// Read previous CLs from the log file.
	prevCLsMap, err := gerrit.ReadLog(logFilePathFlag)
	if err != nil {
		return err
	}

	// Query Gerrit.
	gUrl, err := gerritBaseUrl()
	if err != nil {
		return err
	}
	curCLs, err := jirix.Gerrit(gUrl).Query(queryStringFlag)
	if err != nil {
		return fmt.Errorf("Query(%q) failed: %v", queryStringFlag, err)
	}

	// Write current CLs to the log file.
	err = gerrit.WriteLog(logFilePathFlag, curCLs)
	if err != nil {
		return err
	}

	// Don't send anything if jenkins host is not specified.
	if jenkinsHostFlag == "" {
		printf(jirix.Stdout(), "Not sending CLs to run presubmit tests due to empty Jenkins host.\n")
		return nil
	}

	// Don't send anything if prevCLsMap is empty.
	if len(prevCLsMap) == 0 {
		printf(jirix.Stdout(), "Not sending CLs to run presubmit tests due to empty log file.\n")
		return nil
	}

	// Get new clLists.
	newCLLists, multiPartErrs := gerrit.NewOpenCLs(prevCLsMap, curCLs)
	for _, e := range multiPartErrs {
		printf(jirix.Stderr(), "%v\n", e)

		// Post multi-part errors to gerrit.
		if mpErr, ok := e.(*gerrit.ChangeError); ok {
			clRef := mpErr.CL.Reference()
			msg := fmt.Sprintf("failed to process multi-part CL %s:\n%v\n", clRef, mpErr.Err)
			if postErr := postMessage(jirix, msg, []string{clRef}, false); postErr != nil {
				printf(jirix.Stderr(), "%v\n", postErr)
			}
		}
	}

	// Send the new open CLs one by one to the given Jenkins
	// project to run presubmit-test builds.
	projects, _, err := project.LoadManifest(jirix)
	if err != nil {
		return err
	}
	sender := clsSender{
		clLists:          newCLLists,
		projects:         projects,
		clsSent:          0,
		removeOutdatedFn: removeOutdatedBuilds,
		addPresubmitFn:   addPresubmitTestBuild,
		postMessageFn:    postMessage,
	}
	if err := sender.sendCLListsToPresubmitTest(jirix); err != nil {
		return err
	}
	numSentCLs += sender.clsSent

	// Get all submittable CLs and submit them.
	submittableCLs := getSubmittableCLs(jirix, curCLs)
	if len(submittableCLs) > 0 {
		fmt.Fprintf(jirix.Stdout(), "Submitting CLs...\n")
	}
	for _, curCLList := range submittableCLs {
		if err := submitCLs(jirix, curCLList); err != nil {
			return err
		}
	}

	return nil
}

type clsSender struct {
	clLists          []gerrit.CLList
	projects         project.Projects
	clsSent          int
	removeOutdatedFn func(*jiri.X, clNumberToPatchsetMap) []error
	addPresubmitFn   func(*jiri.X, gerrit.CLList, []string) error
	postMessageFn    func(*jiri.X, string, []string, bool) error
}

// sendCLListsToPresubmitTest sends the given clLists to presubmit-test Jenkins
// job one by one to run presubmit-test builds. It returns how many CLs have
// been sent successfully.
func (s *clsSender) sendCLListsToPresubmitTest(jirix *jiri.X) error {
	for _, curCLList := range s.clLists {
		clListInfo := s.processCLList(jirix, curCLList)
		curCLList = clListInfo.filteredCLList
		if len(curCLList) == 0 {
			printf(jirix.Stdout(), "SKIP: Empty CL set\n")
			continue
		}

		// Don't send curCLList to presubmit-test if at least one of them
		// have PresubmitTest set to none.
		if clListInfo.skipPresubmitTest {
			// Set verified+1 label.
			if err := s.postMessageFn(jirix, "Presubmit tests skipped.\n", clListInfo.refs, true); err != nil {
				return err
			}
			printf(jirix.Stdout(), "SKIP: Add %s (presubmit=none)\n", clListInfo.clString)
			continue
		}

		// Skip if there is no tests to run.
		tests, err := s.getTestsToRun(jirix, clListInfo.projects)
		if err != nil {
			return err
		}
		if len(tests) == 0 {
			// Set verified+1 label when there is no tests to run.
			if err := s.postMessageFn(jirix, "No tests found.\n", clListInfo.refs, true); err != nil {
				return err
			}
			printf(jirix.Stdout(), "SKIP: Add %s (no tests found)\n", clListInfo.clString)
			continue
		}

		// Don't send curCLList to presubmit-test if at least one of them
		// has an non-google owner. Instead, post a link that one of our
		// team members has to click to trigger the presubmit-test manually.
		if clListInfo.hasNonGoogleOwner {
			if err := s.handleNonGoogleOwner(jirix, clListInfo.refs, clListInfo.projects, tests); err != nil {
				return err
			}
			printf(jirix.Stdout(), "SKIP: Add %s (non-google owner)\n", clListInfo.clString)
			continue
		}

		// Check and cancel matched outdated builds.
		for _, err := range s.removeOutdatedFn(jirix, clListInfo.clMap) {
			if err != nil {
				printf(jirix.Stderr(), "%v\n", err)
			}
		}

		// Send curCLList to presubmit-test.
		strCLs := fmt.Sprintf("Add %s", clListInfo.clString)
		if err := s.addPresubmitFn(jirix, curCLList, tests); err != nil {
			printf(jirix.Stdout(), "FAIL: %s\n", strCLs)
			printf(jirix.Stderr(), "addPresubmitTestBuild failed: %v\n", err)
		} else {
			printf(jirix.Stdout(), "PASS: %s\n", strCLs)
			s.clsSent += len(curCLList)
		}
	}
	return nil

}

type processCLListResult struct {
	clMap             clNumberToPatchsetMap
	clString          string
	skipPresubmitTest bool
	hasNonGoogleOwner bool
	projects          []string
	refs              []string
	filteredCLList    gerrit.CLList
}

func (s *clsSender) processCLList(jirix *jiri.X, curCLList gerrit.CLList) *processCLListResult {
	curCLMap := clNumberToPatchsetMap{}
	clStrings := []string{}
	skipPresubmitTest := false
	hasNonGoogleOwner := false
	projects := []string{}
	refs := []string{}
	filteredCLList := gerrit.CLList{}
	for _, curCL := range curCLList {
		// Ignore all CLs that are not in projects identified by the manifestFlag.
		// TODO(jingjin): find a better way so we can remove this check.
		if s.projects != nil && !isKnownProject(jirix, curCL, s.projects) {
			continue
		}
		filteredCLList = append(filteredCLList, curCL)

		cl, patchset, err := gerrit.ParseRefString(curCL.Reference())
		if err != nil {
			printf(jirix.Stderr(), "%v\n", err)
			return nil
		}
		curCLMap[cl] = patchset
		clStrings = append(clStrings, fmt.Sprintf("http://go/vcl/%d/%d", cl, patchset))

		if curCL.PresubmitTest == gerrit.PresubmitTestTypeNone {
			skipPresubmitTest = true
		}

		hasNonGoogleOwner = !checkEmailAddress(curCL.OwnerEmail())

		projects = append(projects, curCL.Project)
		refs = append(refs, curCL.Reference())
	}
	return &processCLListResult{
		clMap:             curCLMap,
		clString:          strings.Join(clStrings, ", "),
		skipPresubmitTest: skipPresubmitTest,
		hasNonGoogleOwner: hasNonGoogleOwner,
		projects:          projects,
		refs:              refs,
		filteredCLList:    filteredCLList,
	}
}

func (s *clsSender) getTestsToRun(jirix *jiri.X, projects []string) ([]string, error) {
	config, err := tooldata.LoadConfig(jirix)
	if err != nil {
		return nil, err
	}
	tmpTests := config.ProjectTests(projects)
	tests := []string{}
	// Append the part suffix to tests that have multiple parts specified in the config file.
	for _, test := range tmpTests {
		if parts := config.TestParts(test); parts != nil {
			for i := 0; i <= len(parts); i++ {
				tests = append(tests, testNameWithPartSuffix(test, i))
			}
		} else {
			tests = append(tests, test)
		}
	}
	sort.Strings(tests)
	return tests, nil
}

func (s *clsSender) handleNonGoogleOwner(jirix *jiri.X, refs, projects, tests []string) error {
	link := genStartPresubmitBuildLink(strings.Join(refs, ":"), strings.Join(projects, ":"), strings.Join(tests, " "))
	message := fmt.Sprintf("A Vanadium team member will manually trigger presubmit tests for this change:\n%s\n", link)
	if err := s.postMessageFn(jirix, message, refs, false); err != nil {
		return err
	}
	return nil
}

// isKnownProject checks whether the given cl's project is in the
// given set of projects.
func isKnownProject(jirix *jiri.X, cl gerrit.Change, projects project.Projects) bool {
	foundProjects := projects.Find(cl.Project)
	if len(foundProjects) == 0 {
		printf(jirix.Stdout(), "project=%q (%s) not found. Skipped.\n", cl.Project, cl.Reference())
		return false
	}
	return true
}

// removeOutdatedBuilds removes all the outdated presubmit-test builds
// that have the given cl number and equal or smaller patchset
// number. Outdated builds include queued builds and ongoing build.
//
// Since this is not a critical operation, we simply print out the
// errors if we see any.
func removeOutdatedBuilds(jirix *jiri.X, cls clNumberToPatchsetMap) (errs []error) {
	collect.Errors(func() error { return removeQueuedOutdatedBuilds(jirix, cls) }, &errs)
	collect.Errors(func() error { return removeOngoingOutdatedBuilds(jirix, cls) }, &errs)
	return
}

func removeQueuedOutdatedBuilds(jirix *jiri.X, cls clNumberToPatchsetMap) error {
	jenkins, err := jirix.Jenkins(jenkinsHostFlag)
	if err != nil {
		return err
	}

	// Get queued outdated builds.
	queuedBuilds, err := jenkins.QueuedBuilds(presubmitTestJobFlag)
	if err != nil {
		return err
	}

	for _, build := range queuedBuilds {
		refs := build.ParseRefs()
		if refs == "" {
			return err
		}
		buildOutdated, err := isBuildOutdated(refs, cls)
		if err != nil {
			return err
		}
		if buildOutdated {
			if err := jenkins.CancelQueuedBuild(fmt.Sprintf("%d", build.Id)); err != nil {
				return err
			}
			printf(jirix.Stdout(), "Cancelled build %s as it is no longer current.\n", refs)
		}
	}
	return nil
}

func removeOngoingOutdatedBuilds(jirix *jiri.X, cls clNumberToPatchsetMap) error {
	jenkins, err := jirix.Jenkins(jenkinsHostFlag)
	if err != nil {
		return err
	}

	buildInfos, err := jenkins.OngoingBuilds(presubmitTestJobFlag)
	if err != nil {
		return err
	}

	for _, buildInfo := range buildInfos {
		if !buildInfo.Building {
			continue
		}
		refs := buildInfo.ParseRefs()
		if refs != "" {
			buildOutdated, err := isBuildOutdated(refs, cls)
			if err != nil {
				fmt.Fprintf(jirix.Stderr(), "%v\n", err)
				continue
			}
			// Cancel outdated running build.
			if buildOutdated {
				if err := jenkins.CancelOngoingBuild(presubmitTestJobFlag, buildInfo.Number); err != nil {
					return err
				}
				printf(jirix.Stdout(), "Cancelled build %s as it is no longer current.\n", refs)
			}
		}
	}
	return nil
}

// isBuildOutdated checks whether a build (identified by the given refs string)
// is older than the cls in newCLs.
// Note that curRefs may contain multiple ref strings separated by ":".
func isBuildOutdated(curRefs string, newCLs clNumberToPatchsetMap) (bool, error) {
	// Parse the refs string into a clNumberToPatchsetMap object.
	curCLs := clNumberToPatchsetMap{}
	refs := strings.Split(curRefs, ":")
	for _, ref := range refs {
		cl, patchset, err := gerrit.ParseRefString(ref)
		if err != nil {
			return false, err
		}
		curCLs[cl] = patchset
	}

	// Check curCLs and newCLs have the same set of cl numbers.
	newCLNumbers := sortedKeys(newCLs)
	if !reflect.DeepEqual(sortedKeys(curCLs), newCLNumbers) {
		// curCLs are outdated when curCLs and newCLs have overlapping refs.
		// For example: curCLs = {1000/1}, and newCLs = {1000/2, 2000/1}.
		// In this case, 1000/1 becomes part of the MultiPart CLs, which makes
		// 1000/1 outdated.
		for curCLNumber, curPatchset := range curCLs {
			if newPatchset, ok := newCLs[curCLNumber]; ok && newPatchset >= curPatchset {
				return true, nil
			}
		}
		return false, nil
	}

	// Check patchsets.
	outdated := true
	for _, clNumber := range newCLNumbers {
		curPatchset := curCLs[clNumber]
		newPatchset := newCLs[clNumber]
		if newPatchset < curPatchset {
			outdated = false
			break
		}
	}
	return outdated, nil
}

func sortedKeys(cls clNumberToPatchsetMap) []int {
	keys := []int{}
	for k := range cls {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

// addPresubmitTestBuild uses Jenkins' remote access API to add a build for
// a set of open CLs to run presubmit tests.
func addPresubmitTestBuild(jirix *jiri.X, cls gerrit.CLList, tests []string) error {
	jenkins, err := jirix.Jenkins(jenkinsHostFlag)
	if err != nil {
		return err
	}

	refs, projects := []string{}, []string{}
	for _, cl := range cls {
		refs = append(refs, cl.Reference())
		projects = append(projects, cl.Project)
	}
	if err := jenkins.AddBuildWithParameter(presubmitTestJobFlag, url.Values{
		"REFS":     {strings.Join(refs, ":")},
		"PROJECTS": {strings.Join(projects, ":")},
		// Separating by spaces is required by the Dynamic Axis plugin used in the
		// new presubmit test target.
		"TESTS": {strings.Join(tests, " ")},
	}); err != nil {
		return err
	}
	return nil
}

// checkEmailAddress checks whether the given email address is from Google or
// is whitelisted.
func checkEmailAddress(email string) bool {
	fromGoogle := strings.HasSuffix(email, "@google.com")
	whiteListed := false
	for _, we := range emailWhitelist {
		if we == strings.ToLower(email) {
			whiteListed = true
			break
		}
	}
	return fromGoogle || whiteListed
}
