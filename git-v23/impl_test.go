package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"v.io/tools/lib/gerrit"
	"v.io/tools/lib/gitutil"
	"v.io/tools/lib/util"
)

// assertCommitCount asserts that the commit count between two
// branches matches the expectedCount.
func assertCommitCount(t *testing.T, ctx *util.Context, branch, baseBranch string, expectedCount int) {
	got, err := ctx.Git().CountCommits(branch, baseBranch)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if want := 1; got != want {
		t.Fatalf("unexpected number of commits: got %v, want %v", got, want)
	}
}

// assertFileContent asserts that the content of the given file
// matches the expected content.
func assertFileContent(t *testing.T, ctx *util.Context, file, want string) {
	got, err := ioutil.ReadFile(file)
	if err != nil {
		t.Fatalf("ReadFile(%v) failed: %v", file, err)
	}
	if string(got) != want {
		t.Fatalf("unexpected content of file %v: got %v, want %v", file, got, want)
	}
}

// assertFilesCommitted asserts that the files exist and are committed
// in the current branch.
func assertFilesCommitted(t *testing.T, ctx *util.Context, files []string) {
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			if os.IsNotExist(err) {
				t.Fatalf("expected file %v to exist but it did not", file)
			}
			t.Fatalf("%v", err)
		}
		if !ctx.Git().IsFileCommitted(file) {
			t.Fatalf("expected file %v to be committed but it is not", file)
		}
	}
}

// assertFilesNotCommitted asserts that the files exist and are *not*
// committed in the current branch.
func assertFilesNotCommitted(t *testing.T, ctx *util.Context, files []string) {
	for _, file := range files {
		if _, err := os.Stat(file); err != nil {
			if os.IsNotExist(err) {
				t.Fatalf("expected file %v to exist but it did not", file)
			}
			t.Fatalf("%v", err)
		}
		if ctx.Git().IsFileCommitted(file) {
			t.Fatalf("expected file %v not to be committed but it is", file)
		}
	}
}

// assertFilesPushedToRef asserts that the given files have been
// pushed to the given remote repository reference.
func assertFilesPushedToRef(t *testing.T, ctx *util.Context, repoPath, gerritPath, pushedRef string, files []string) {
	if err := ctx.Run().Chdir(gerritPath); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", gerritPath, err)
	}
	assertCommitCount(t, ctx, pushedRef, "master", 1)
	if err := ctx.Git().CheckoutBranch(pushedRef, !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	assertFilesCommitted(t, ctx, files)
	if err := ctx.Run().Chdir(repoPath); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", repoPath, err)
	}
}

// assertStashSize asserts that the stash size matches the expected
// size.
func assertStashSize(t *testing.T, ctx *util.Context, want int) {
	got, err := ctx.Git().StashSize()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if got != want {
		t.Fatalf("unxpected stash size: got %v, want %v", got, want)
	}
}

// commitFiles commits the given files into to current branch.
func commitFiles(t *testing.T, ctx *util.Context, fileNames []string) {
	// Create and commit the files one at a time.
	for _, fileName := range fileNames {
		fileContent := "This is file " + fileName
		if err := ctx.Run().WriteFile(fileName, []byte(fileContent), 0644); err != nil {
			t.Fatalf("%v", err)
		}
		commitMessage := "Commit " + fileName
		if err := ctx.Git().CommitFile(fileName, commitMessage); err != nil {
			t.Fatalf("%v", err)
		}
	}
}

func createConfig(t *testing.T, ctx *util.Context, rootDir string) {
	configFile, err := util.ConfigFile("common")
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := ctx.Run().MkdirAll(filepath.Dir(configFile), os.FileMode(0755)); err != nil {
		t.Fatalf("%v", err)
	}
	config := util.Config{}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := ctx.Run().WriteFile(configFile, data, os.FileMode(0644)); err != nil {
		t.Fatalf("WriteFile(%v) failed: %v", configFile, err)
	}
}

func createTestGoDependencyPackages(t *testing.T, ctx *util.Context, rootDir string) {
	fooDir := filepath.Join(rootDir, "src", "v.io", "foo")
	if err := ctx.Run().MkdirAll(fooDir, os.FileMode(0755)); err != nil {
		t.Fatalf("MkdirAll(%v) failed: %v", fooDir, err)
	}
	fooFile := filepath.Join(fooDir, "foo.go")
	fooData := `package foo

func Foo() string {
	return "hello"
}
`
	if err := ctx.Run().WriteFile(fooFile, []byte(fooData), os.FileMode(0644)); err != nil {
		t.Fatalf("WriteFile(%v) failed: %v", fooFile, err)
	}
	if err := ctx.Git().CommitFile(fooFile, "commit foo.go"); err != nil {
		t.Fatalf("%v", err)
	}
	barDir := filepath.Join(rootDir, "src", "v.io", "bar")
	if err := ctx.Run().MkdirAll(barDir, os.FileMode(0755)); err != nil {
		t.Fatalf("MkdirAll(%v) failed: %v", barDir, err)
	}
	barFile := filepath.Join(barDir, "bar.go")
	barData := `package bar

import "v.io/foo"

func Bar() string {
	return foo.Foo()
}
`
	if err := ctx.Run().WriteFile(barFile, []byte(barData), os.FileMode(0644)); err != nil {
		t.Fatalf("WriteFile(%v) failed: %v", barFile, err)
	}
	if err := ctx.Git().CommitFile(barFile, "commit bar.go"); err != nil {
		t.Fatalf("%v", err)
	}
}

func createTestGoDependencyConstraint(t *testing.T, ctx *util.Context, rootDir, command string) {
	depFile := filepath.Join(rootDir, "src", "v.io", "foo", "GO.PACKAGE")
	depData := `{
  "dependencies": {
    "incoming": [
      {"` + command + `": "..."}
    ]
  }
}
`
	if err := ctx.Run().WriteFile(depFile, []byte(depData), os.FileMode(0644)); err != nil {
		t.Fatalf("WriteFile(%v) failed: %v", depFile, err)
	}
	if err := ctx.Git().CommitFile(depFile, "commit GO.PACKAGE"); err != nil {
		t.Fatalf("%v", err)
	}
}

// createRepo creates a new repository in the given working directory.
func createRepo(t *testing.T, ctx *util.Context, workingDir, prefix string) string {
	repoPath, err := ctx.Run().TempDir(workingDir, "repo-"+prefix)
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}
	if err := os.Chmod(repoPath, 0777); err != nil {
		t.Fatalf("Chmod(%v) failed: %v", repoPath, err)
	}
	if err := ctx.Git().Init(repoPath); err != nil {
		t.Fatalf("%v", err)
	}
	return repoPath
}

// Simple commit-msg hook that adds a fake Change Id.
var commitMsgHook string = `
#!/bin/sh
MSG="$1"
echo "Change-Id: I0000000000000000000000000000000000000000" >> $MSG
`

// installCommitMsgHook links the gerrit commit-msg hook into a different repo.
func installCommitMsgHook(t *testing.T, ctx *util.Context, repoPath string) {
	hookLocation := path.Join(repoPath, ".git/hooks/commit-msg")
	if err := ctx.Run().WriteFile(hookLocation, []byte(commitMsgHook), 0755); err != nil {
		t.Fatalf("WriteFile(%v) failed: %v", hookLocation, err)
	}
}

// createTestRepos sets up three local repositories: origin, gerrit,
// and the main test repository which pulls from origin and can push
// to gerrit.
func createTestRepos(t *testing.T, ctx *util.Context, workingDir string) (string, string, string) {
	// Create origin.
	originPath := createRepo(t, ctx, workingDir, "origin")
	if err := ctx.Run().Chdir(originPath); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", originPath, err)
	}
	if err := ctx.Git().CommitWithMessage("initial commit"); err != nil {
		t.Fatalf("%v", err)
	}
	// Create test repo.
	repoPath := createRepo(t, ctx, workingDir, "test")
	if err := ctx.Run().Chdir(repoPath); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", repoPath, err)
	}
	if err := ctx.Git().AddRemote("origin", originPath); err != nil {
		t.Fatalf("%v", err)
	}
	if err := ctx.Git().Pull("origin", "master"); err != nil {
		t.Fatalf("%v", err)
	}
	// Add Gerrit remote.
	gerritPath := createRepo(t, ctx, workingDir, "gerrit")
	if err := ctx.Run().Chdir(gerritPath); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", gerritPath, err)
	}
	if err := ctx.Git().AddRemote("origin", originPath); err != nil {
		t.Fatalf("%v", err)
	}
	if err := ctx.Git().Pull("origin", "master"); err != nil {
		t.Fatalf("%v", err)
	}
	// Switch back to test repo.
	if err := ctx.Run().Chdir(repoPath); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", repoPath, err)
	}
	return repoPath, originPath, gerritPath
}

// setup creates a set up for testing the review tool.
func setup(t *testing.T, ctx *util.Context, installHook bool) (string, string, string, string) {
	workingDir, err := ctx.Run().TempDir("", "test-git-v23-review")
	if err != nil {
		t.Fatalf("TempDir() failed: %v", err)
	}
	repoPath, originPath, gerritPath := createTestRepos(t, ctx, workingDir)
	if installHook == true {
		for _, path := range []string{repoPath, originPath, gerritPath} {
			installCommitMsgHook(t, ctx, path)
		}
	}
	if err := ctx.Run().Chdir(repoPath); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", repoPath, err)
	}
	return workingDir, repoPath, originPath, gerritPath
}

// teardown cleans up the set up for testing the review tool.
func teardown(t *testing.T, ctx *util.Context, workingDir string) {
	if err := ctx.Run().RemoveAll(workingDir); err != nil {
		t.Fatalf("RemoveAll(%v) failed: %v", workingDir, err)
	}
}

// TestCleanupClean checks that cleanup succeeds if the branch to be
// cleaned up has been merged with the master.
func TestCleanupClean(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, _, _, _ := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	commitFiles(t, ctx, []string{"file1", "file2"})
	if err := ctx.Git().CheckoutBranch("master", !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	if err := ctx.Git().Merge(branch, true); err != nil {
		t.Fatalf("%v", err)
	}
	if err := ctx.Git().Commit(); err != nil {
		t.Fatalf("%v", err)
	}
	if err := cleanup(ctx, []string{branch}); err != nil {
		t.Fatalf("cleanup() failed: %v", err)
	}
	if ctx.Git().BranchExists(branch) {
		t.Fatalf("cleanup failed to remove the feature branch")
	}
}

// TestCleanupDirty checks that cleanup is a no-op if the branch to be
// cleaned up has unmerged changes.
func TestCleanupDirty(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, _, _, _ := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	files := []string{"file1", "file2"}
	commitFiles(t, ctx, files)
	if err := ctx.Git().CheckoutBranch("master", !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	if err := cleanup(ctx, []string{branch}); err == nil {
		t.Fatalf("cleanup did not fail when it should")
	}
	if err := ctx.Git().CheckoutBranch(branch, !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	assertFilesCommitted(t, ctx, files)
}

// TestCreateReviewBranch checks that the temporary review branch is
// created correctly.
func TestCreateReviewBranch(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, _, _, _ := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	files := []string{"file1", "file2", "file3"}
	commitFiles(t, ctx, files)
	draft, edit, repo, reviewers, ccs := false, false, "", "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if expected, got := branch+"-REVIEW", review.reviewBranch; expected != got {
		t.Fatalf("Unexpected review branch name: expected %v, got %v", expected, got)
	}
	commitMessage := "squashed commit"
	if err := review.createReviewBranch(commitMessage); err != nil {
		t.Fatalf("%v", err)
	}
	// Verify that the branch exists.
	if !ctx.Git().BranchExists(review.reviewBranch) {
		t.Fatalf("review branch not found")
	}
	if err := ctx.Git().CheckoutBranch(review.reviewBranch, !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	assertCommitCount(t, ctx, review.reviewBranch, "master", 1)
	assertFilesCommitted(t, ctx, files)
}

// TestCreateReviewBranchWithEmptyChange checks that running
// createReviewBranch() on a branch with no changes will result in an
// EmptyChangeError.
func TestCreateReviewBranchWithEmptyChange(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, _, _, _ := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	draft, edit, repo, reviewers, ccs := false, false, branch, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	commitMessage := "squashed commit"
	err = review.createReviewBranch(commitMessage)
	if err == nil {
		t.Fatalf("creating a review did not fail when it should")
	}
	if _, ok := err.(emptyChangeError); !ok {
		t.Fatalf("unexpected error type: %v", err)
	}
}

func TestGoDependencyError(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, repoPath, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	oldRoot := os.Getenv("VANADIUM_ROOT")
	if err := os.Setenv("VANADIUM_ROOT", workingDir); err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Setenv("VANADIUM_ROOT", oldRoot)
	oldGoPath := os.Getenv("GOPATH")
	if err := os.Setenv("GOPATH", repoPath); err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Setenv("GOPATH", oldGoPath)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	createConfig(t, ctx, workingDir)
	createTestGoDependencyPackages(t, ctx, repoPath)
	createTestGoDependencyConstraint(t, ctx, repoPath, "deny")
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := review.checkGoDependencies(); err == nil {
		t.Fatalf("go format check did not fail when it should")
	} else if _, ok := err.(goDependencyError); !ok {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGoDependencyOK(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, repoPath, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	oldRoot := os.Getenv("VANADIUM_ROOT")
	if err := os.Setenv("VANADIUM_ROOT", workingDir); err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Setenv("VANADIUM_ROOT", oldRoot)
	oldGoPath := os.Getenv("GOPATH")
	if err := os.Setenv("GOPATH", repoPath); err != nil {
		t.Fatalf("%v", err)
	}
	defer os.Setenv("GOPATH", oldGoPath)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	createConfig(t, ctx, workingDir)
	createTestGoDependencyPackages(t, ctx, repoPath)
	createTestGoDependencyConstraint(t, ctx, repoPath, "allow")
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := review.checkGoDependencies(); err != nil {
		t.Fatalf("go dependency check failed: %v", err)
	}
}

func TestGoFormatError(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, _, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	file, fileContent := "file.go", ` package main

func main() {}
`
	if err := ctx.Run().WriteFile(file, []byte(fileContent), 0644); err != nil {
		t.Fatalf("WriteFile(%v, %v) failed: %v", file, fileContent, err)
	}
	commitMessage := "Commit " + file
	if err := ctx.Git().CommitFile(file, commitMessage); err != nil {
		t.Fatalf("%v", err)
	}
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := review.checkGoFormat(); err == nil {
		t.Fatalf("go format check did not fail when it should")
	} else if _, ok := err.(goFormatError); !ok {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGoFormatOK(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, _, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	file, fileContent := "file.go", `package main

func main() {}
`
	if err := ctx.Run().WriteFile(file, []byte(fileContent), 0644); err != nil {
		t.Fatalf("WriteFile(%v, %v) failed: %v", file, fileContent, err)
	}
	commitMessage := "Commit " + file
	if err := ctx.Git().CommitFile(file, commitMessage); err != nil {
		t.Fatalf("%v", err)
	}
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := review.checkGoFormat(); err != nil {
		t.Fatalf("go format check failed: %v", err)
	}
}

// TestSendReview checks the various options for sending a review.
func TestSendReview(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, repoPath, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	files := []string{"file1"}
	commitFiles(t, ctx, files)
	{
		// Test with draft = false, no reviewiers, and no ccs.
		draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("failed to send a review: %v", err)
		}
		expectedRef := gerrit.Reference(review.draft, review.reviewers, review.ccs, review.branch)
		assertFilesPushedToRef(t, ctx, repoPath, gerritPath, expectedRef, files)
	}
	{
		// Test with draft = true, no reviewers, and no ccs.
		draft, edit, repo, reviewers, ccs := true, false, gerritPath, "", ""
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("failed to send a review: %v", err)
		}
		expectedRef := gerrit.Reference(draft, reviewers, ccs, review.branch)
		assertFilesPushedToRef(t, ctx, repoPath, gerritPath, expectedRef, files)
	}
	{
		// Test with draft = false, reviewers, and no ccs.
		draft, edit, repo, reviewers, ccs := false, false, gerritPath, "reviewer1,reviewer2@example.org", ""
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("failed to send a review: %v", err)
		}
		expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
		assertFilesPushedToRef(t, ctx, repoPath, gerritPath, expectedRef, files)
	}
	{
		// Test with draft = true, reviewers, and ccs.
		draft, edit, repo, reviewers, ccs := true, false, gerritPath, "reviewer3@example.org,reviewer4", "cc1@example.org,cc2"
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("failed to send a review: %v", err)
		}
		expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
		assertFilesPushedToRef(t, ctx, repoPath, gerritPath, expectedRef, files)
	}
}

// TestSendReviewNoChangeID checks that review.send() correctly errors when
// not run with a commit hook that adds a Change-Id.
func TestSendReviewNoChangeID(t *testing.T) {
	ctx := util.DefaultContext()
	// Pass 'false' to setup so it doesn't install the commit-msg hook.
	workingDir, _, _, gerritPath := setup(t, ctx, false)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	commitFiles(t, ctx, []string{"file1"})
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = review.send()
	if err == nil {
		t.Fatalf("sending a review did not fail when it should")
	}
	if _, ok := err.(noChangeIDError); !ok {
		t.Fatalf("unexpected error type: %v", err)
	}
}

// TestEndToEnd checks the end-to-end functionality of the review tool.
func TestEndToEnd(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, repoPath, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	files := []string{"file1", "file2", "file3"}
	commitFiles(t, ctx, files)
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	depcopFlag = false
	review.run()
	expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
	assertFilesPushedToRef(t, ctx, repoPath, gerritPath, expectedRef, files)
}

// TestDirtyBranch checks that the tool correctly handles unstaged and
// untracked changes in a working branch with stashed changes.
func TestDirtyBranch(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, repoPath, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	files := []string{"file1", "file2", "file3"}
	commitFiles(t, ctx, files)
	assertStashSize(t, ctx, 0)
	stashedFile, stashedFileContent := "stashed-file", "stashed-file content"
	if err := ctx.Run().WriteFile(stashedFile, []byte(stashedFileContent), 0644); err != nil {
		t.Fatalf("WriteFile(%v, %v) failed: %v", stashedFile, stashedFileContent, err)
	}
	if err := ctx.Git().Add(stashedFile); err != nil {
		t.Fatalf("%v", err)
	}
	if _, err := ctx.Git().Stash(); err != nil {
		t.Fatalf("%v", err)
	}
	assertStashSize(t, ctx, 1)
	modifiedFile, modifiedFileContent := "modified-file", "modified-file content"
	if err := ctx.Run().WriteFile(modifiedFile, []byte(modifiedFileContent), 0644); err != nil {
		t.Fatalf("WriteFile(%v, %v) failed: %v", modifiedFile, modifiedFileContent, err)
	}
	stagedFile, stagedFileContent := "staged-file", "staged-file content"
	if err := ctx.Run().WriteFile(stagedFile, []byte(stagedFileContent), 0644); err != nil {
		t.Fatalf("WriteFile(%v, %v) failed: %v", stagedFile, stagedFileContent, err)
	}
	if err := ctx.Git().Add(stagedFile); err != nil {
		t.Fatalf("%v", err)
	}
	untrackedFile, untrackedFileContent := "untracked-file", "untracked-file content"
	if err := ctx.Run().WriteFile(untrackedFile, []byte(untrackedFileContent), 0644); err != nil {
		t.Fatalf("WriteFile(%v, %t) failed: %v", untrackedFile, untrackedFileContent, err)
	}
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	depcopFlag = false
	review.run()
	expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
	assertFilesPushedToRef(t, ctx, repoPath, gerritPath, expectedRef, files)
	assertFilesNotCommitted(t, ctx, []string{stagedFile})
	assertFilesNotCommitted(t, ctx, []string{untrackedFile})
	assertFileContent(t, ctx, modifiedFile, modifiedFileContent)
	assertFileContent(t, ctx, stagedFile, stagedFileContent)
	assertFileContent(t, ctx, untrackedFile, untrackedFileContent)
	assertStashSize(t, ctx, 1)
	if err := ctx.Git().StashPop(); err != nil {
		t.Fatalf("%v", err)
	}
	assertStashSize(t, ctx, 0)
	assertFilesNotCommitted(t, ctx, []string{stashedFile})
	assertFileContent(t, ctx, stashedFile, stashedFileContent)
}

// TestRunInSubdirectory checks that the command will succeed when run from
// within a subdirectory of a branch that does not exist on master branch, and
// will return the user to the subdirectory after completion.
func TestRunInSubdirectory(t *testing.T) {
	ctx := util.DefaultContext()
	workingDir, repoPath, _, gerritPath := setup(t, ctx, true)
	defer teardown(t, ctx, workingDir)
	branch := "my-branch"
	if err := ctx.Git().CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("%v", err)
	}
	subdir := "sub/directory"
	subdirPerms := os.FileMode(0744)
	if err := ctx.Run().MkdirAll(subdir, subdirPerms); err != nil {
		t.Fatalf("MkdirAll(%v, %v) failed: %v", subdir, subdirPerms, err)
	}
	files := []string{path.Join(subdir, "file1")}
	commitFiles(t, ctx, files)
	if err := ctx.Run().Chdir(subdir); err != nil {
		t.Fatalf("Chdir(%v) failed: %v", subdir, err)
	}
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	depcopFlag = false
	review.run()
	path := path.Join(repoPath, subdir)
	want, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%v) failed: %v", path, err)
	}
	workingDir, err = os.Getwd()
	if err != nil {
		t.Fatalf("%v", err)
	}
	got, err := filepath.EvalSymlinks(workingDir)
	if err != nil {
		t.Fatalf("EvalSymlinks(%v) failed: %v", workingDir, err)
	}
	if got != want {
		t.Fatalf("unexpected working directory: got %v, want %v", got, want)
	}
	expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
	assertFilesPushedToRef(t, ctx, repoPath, gerritPath, expectedRef, files)
}