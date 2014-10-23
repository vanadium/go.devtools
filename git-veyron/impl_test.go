package main

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	"tools/lib/cmdline"
	"tools/lib/gerrit"
	"tools/lib/gitutil"
	"tools/lib/runutil"
	"tools/lib/util"
)

var (
	run = runutil.New(true, os.Stdout)
	git = gitutil.New(run)
)

// assertCommitCount asserts that the commit count between two
// branches matches the expectedCount.
func assertCommitCount(t *testing.T, branch, baseBranch string, expectedCount int) {
	commitCount, err := git.CountCommits(branch, baseBranch)
	if err != nil {
		t.Fatalf("CountCommits(%v, %v) failed: %v", branch, baseBranch, err)
	}
	expectedCommitCount := 1
	if commitCount != expectedCommitCount {
		t.Fatalf("Unexpected number of commits: expected %v, got %v", expectedCommitCount, commitCount)
	}
}

// assertFileContent asserts that the content of the given file
// matches the expected content.
func assertFileContent(t *testing.T, file, expectedContent string) {
	actualContent := readFile(file)
	if expectedContent != actualContent {
		t.Fatalf("Expected file %v to contain %v, but it actually contains %v", file, expectedContent, actualContent)
	}
}

// assertFilesCommitted asserts that the files exist and are committed
// in the current branch.
func assertFilesCommitted(t *testing.T, files []string) {
	for _, fileName := range files {
		if !fileExists(fileName) {
			t.Fatalf("Expected file %v to exist but it did not.", fileName)
		}
		if !git.IsFileCommitted(fileName) {
			t.Fatalf("Expected file %v to be committed but it is not.", fileName)
		}
	}
}

// assertFilesNotCommitted asserts that the files exist and are *not*
// committed in the current branch.
func assertFilesNotCommitted(t *testing.T, files []string) {
	for _, fileName := range files {
		if !fileExists(fileName) {
			t.Fatalf("Expected file %v to exist but it did not.", fileName)
		}
		if git.IsFileCommitted(fileName) {
			t.Fatalf("Expected file %v not to be committed but it is.", fileName)
		}
	}
}

// assertFilesPushedToRef asserts that the given files have been
// pushed to the given remote repository reference.
func assertFilesPushedToRef(t *testing.T, repoPath, gerritPath, pushedRef string, files []string) {
	if err := os.Chdir(gerritPath); err != nil {
		t.Fatalf("os.Chdir(%v) failed: %v", gerritPath, err)
	}
	assertCommitCount(t, pushedRef, "master", 1)
	if err := git.CheckoutBranch(pushedRef, !gitutil.Force); err != nil {
		t.Fatalf("CheckoutBranch(%v, %v) failed: %v", pushedRef, !gitutil.Force, err)
	}
	assertFilesCommitted(t, files)
	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("os.Chdir(%v) failed: %v", repoPath, err)
	}
}

// assertStashSize asserts that the stash size matches the expected
// size.
func assertStashSize(t *testing.T, expectedStashSize int) {
	actualStashSize, err := git.StashSize()
	if err != nil {
		t.Fatalf("StashSize() failed: %v", err)
	}
	if actualStashSize != expectedStashSize {
		t.Fatalf("Expected stash size to be %v, but actual stash size is %v", expectedStashSize, actualStashSize)
	}
}

// commitFiles commits the given files into to current branch.
func commitFiles(fileNames []string) error {
	// Create and commit the files one at a time.
	for _, fileName := range fileNames {
		fileContent := "This is file " + fileName
		if err := writeFile(fileName, fileContent); err != nil {
			return err
		}
		commitMessage := "Commit " + fileName
		if err := git.CommitFile(fileName, commitMessage); err != nil {
			return err
		}
	}
	return nil
}

// createRepo creates a new repository in the given working directory.
func createRepo(workingDir, prefix string) (string, error) {
	repoPath, err := ioutil.TempDir(workingDir, "repo-"+prefix)
	if err != nil {
		return repoPath, err
	}
	if err := os.Chmod(repoPath, 0777); err != nil {
		return repoPath, err
	}
	if err := git.Init(repoPath); err != nil {
		return repoPath, err
	}
	return repoPath, nil
}

// Simple commit-msg hook that adds a fake Change Id.
var commitMsgHook string = `
#!/bin/sh
MSG="$1"
echo "Change-Id: I0000000000000000000000000000000000000000" >> $MSG
`

// installCommitMsgHook links the gerrit commit-msg hook into a different repo.
func installCommitMsgHook(repoPath string) error {
	hookLocation := path.Join(repoPath, ".git/hooks/commit-msg")
	return writeFileExecutable(hookLocation, commitMsgHook)
}

// createTestRepos sets up three local repositories: origin, gerrit,
// and the main test repository which pulls from origin and can push
// to gerrit.
func createTestRepos(workingDir string) (string, string, string, error) {
	// Create origin.
	originPath, err := createRepo(workingDir, "origin")
	if err != nil {
		return "", "", "", err
	}
	if err := os.Chdir(originPath); err != nil {
		return "", "", "", err
	}
	if err := git.CommitWithMessage("initial commit"); err != nil {
		return "", "", "", err
	}
	// Create test repo.
	repoPath, err := createRepo(workingDir, "test")
	if err != nil {
		return "", "", "", err
	}
	if err := os.Chdir(repoPath); err != nil {
		return "", "", "", err
	}
	if err := git.AddRemote("origin", originPath); err != nil {
		return "", "", "", err
	}
	if err := git.Pull("origin", "master"); err != nil {
		return "", "", "", err
	}
	// Add Gerrit remote.
	gerritPath, err := createRepo(workingDir, "gerrit")
	if err != nil {
		return "", "", "", err
	}
	if err := os.Chdir(gerritPath); err != nil {
		return "", "", "", err
	}
	if err := git.AddRemote("origin", originPath); err != nil {
		return "", "", "", err
	}
	if err := git.Pull("origin", "master"); err != nil {
		return "", "", "", err
	}
	// Switch back to test repo.
	if err := os.Chdir(repoPath); err != nil {
		return "", "", "", err
	}
	return repoPath, originPath, gerritPath, nil
}

// setup creates a set up for testing the review tool.
func setup(t *testing.T, installHook bool) (string, string, string, string) {
	workingDir, err := ioutil.TempDir("", "test-git-veyron-review")
	if err != nil {
		t.Fatalf("Error creating working directory: %v", err)
	}
	repoPath, originPath, gerritPath, err := createTestRepos(workingDir)
	if err != nil {
		t.Fatalf("Error creating repo: %v", err)
	}
	if installHook == true {
		for _, path := range []string{repoPath, originPath, gerritPath} {
			if err := installCommitMsgHook(path); err != nil {
				t.Fatalf("Error installing commit-msg hook: %v", err)
			}
		}
	}
	if err := os.Chdir(repoPath); err != nil {
		t.Fatalf("os.Chdir(%v) failed: %v", repoPath, err)
	}
	return workingDir, repoPath, originPath, gerritPath
}

// teardown cleans up the set up for testing the review tool.
func teardown(t *testing.T, workingDir string) {
	if err := os.RemoveAll(workingDir); err != nil {
		t.Fatalf("os.RemoveAll(%v) failed: ", workingDir, err)
	}
}

// TestCleanupClean checks that cleanup succeeds if the branch to be
// cleaned up has been merged with the master.
func TestCleanupClean(t *testing.T) {
	workingDir, _, _, _ := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	file1 := []string{"file1"}
	if err := commitFiles(file1); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", file1, err)
	}
	file2 := []string{"file2"}
	if err := commitFiles(file2); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", file2, err)
	}
	if err := git.CheckoutBranch("master", !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	if err := git.Merge(branch, true); err != nil {
		t.Fatalf("%v", err)
	}
	if err := git.Commit(); err != nil {
		t.Fatalf("%v", err)
	}
	testCmd := cmdline.Command{}
	testCmd.Init(nil, os.Stdout, os.Stderr)
	if err := cleanup(&testCmd, git, run, []string{branch}); err != nil {
		t.Fatalf("cleanup() failed: %v", err)
	}
	if git.BranchExists(branch) {
		t.Fatalf("cleanup failed to remove the feature branch")
	}
}

// TestCleanupDirty checks that cleanup is a no-op if the branch to be
// cleaned up has unmerged changes.
func TestCleanupDirty(t *testing.T) {
	workingDir, _, _, _ := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	files := []string{"file1", "file2"}
	if err := commitFiles(files); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", files, err)
	}
	if err := git.CheckoutBranch("master", !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	testCmd := cmdline.Command{}
	testCmd.Init(nil, os.Stdout, os.Stderr)
	if err := cleanup(&testCmd, git, run, []string{branch}); err == nil {
		t.Fatalf("cleanup did not fail when it should")
	}
	if err := git.CheckoutBranch(branch, !gitutil.Force); err != nil {
		t.Fatalf("%v", err)
	}
	assertFilesCommitted(t, files)
}

// TestCreateReviewBranch checks that the temporary review branch is
// created correctly.
func TestCreateReviewBranch(t *testing.T) {
	workingDir, _, _, _ := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	files := []string{"file1", "file2", "file3"}
	if err := commitFiles(files); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", files, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
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
		t.Fatalf("createReviewBranch() failed: %v", err)
	}
	// Verify that the branch exists.
	if !git.BranchExists(review.reviewBranch) {
		t.Fatalf("Expected createReviewBranch() to create a new branch, but it did not.")
	}
	if err := git.CheckoutBranch(review.reviewBranch, !gitutil.Force); err != nil {
		t.Fatalf("CheckoutBranch(%v, %v) failed: %v", review, !gitutil.Force, err)
	}
	assertCommitCount(t, review.reviewBranch, "master", 1)
	assertFilesCommitted(t, files)
}

// TestCreateReviewBranchWithEmptyChange checks that running
// createReviewBranch() on a branch with no changes will result in an
// EmptyChangeError.
func TestCreateReviewBranchWithEmptyChange(t *testing.T) {
	workingDir, _, _, _ := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	draft, edit, repo, reviewers, ccs := false, false, branch, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	commitMessage := "squashed commit"
	err = review.createReviewBranch(commitMessage)
	if err == nil {
		t.Fatal("Expected createReviewBranch() on an branch with no new commits to fail but it did not.")
	}
	if _, ok := err.(emptyChangeError); !ok {
		t.Fatalf("Expected createReviewBranch() on an branch with no new commits to fail with EmptyChangeError but instead got %v", err)
	}
}

func TestGoFormatError(t *testing.T) {
	workingDir, _, _, gerritPath := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	file, fileContent := "file.go", ` package main

func main() {}
`
	if err := writeFile(file, fileContent); err != nil {
		t.Fatalf("writeFile(%v, %v) failed: %v", file, fileContent, err)
	}
	commitMessage := "Commit " + file
	if err := git.CommitFile(file, commitMessage); err != nil {
		t.Fatalf("CommitFile(%v, %v) failed: %v", file, commitMessage, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := review.checkGoFormat(); err == nil {
		t.Fatalf("checkGoFormat() did not fail")
	}
}

func TestGoFormatOK(t *testing.T) {
	workingDir, _, _, gerritPath := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	file, fileContent := "file.go", `package main

func main() {}
`
	if err := writeFile(file, fileContent); err != nil {
		t.Fatalf("writeFile(%v, %v) failed: %v", file, fileContent, err)
	}
	commitMessage := "Commit " + file
	if err := git.CommitFile(file, commitMessage); err != nil {
		t.Fatalf("CommitFile(%v, %v) failed: %v", file, commitMessage, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	if err := review.checkGoFormat(); err != nil {
		t.Fatalf("checkGoFormat() failed: %v", err)
	}
}

// TestSendReview checks the various options for sending a review.
func TestSendReview(t *testing.T) {
	workingDir, repoPath, _, gerritPath := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	files := []string{"file1"}
	if err := commitFiles(files); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", files, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	{
		// Test with draft = false, no reviewiers, and no ccs.
		draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("send() failed: %v", err)
		}
		expectedRef := gerrit.Reference(review.draft, review.reviewers, review.ccs, review.branch)
		assertFilesPushedToRef(t, repoPath, gerritPath, expectedRef, files)
	}
	{
		// Test with draft = true, no reviewers, and no ccs.
		draft, edit, repo, reviewers, ccs := true, false, gerritPath, "", ""
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("send() failed: %v", err)
		}
		expectedRef := gerrit.Reference(draft, reviewers, ccs, review.branch)
		assertFilesPushedToRef(t, repoPath, gerritPath, expectedRef, files)
	}
	{
		// Test with draft = false, reviewers, and no ccs.
		draft, edit, repo, reviewers, ccs := false, false, gerritPath, "reviewer1,reviewer2@example.org", ""
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("send() failed: %v", err)
		}
		expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
		assertFilesPushedToRef(t, repoPath, gerritPath, expectedRef, files)
	}
	{
		// Test with draft = true, reviewers, and ccs.
		draft, edit, repo, reviewers, ccs := true, false, gerritPath, "reviewer3@example.org,reviewer4", "cc1@example.org,cc2"
		review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if err := review.send(); err != nil {
			t.Fatalf("send() failed: %v", err)
		}
		expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
		assertFilesPushedToRef(t, repoPath, gerritPath, expectedRef, files)
	}
}

// TestSendReviewNoChangeID checks that review.send() correctly errors when
// not run with a commit hook that adds a Change-Id.
func TestSendReviewNoChangeID(t *testing.T) {
	// Pass 'false' to setup so it doesn't install the commit-msg hook.
	workingDir, _, _, gerritPath := setup(t, false)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	files := []string{"file1"}
	if err := commitFiles(files); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", files, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	err = review.send()
	if err == nil {
		t.Fatal("Expected review.send() on an repo with no gerrit commit-msg hook to fail but it did not.")
	}
	if _, ok := err.(noChangeIDError); !ok {
		t.Fatal("Expected review.send() on an repo with no gerrit commit-msg hook to fail with NoChangeIDError but instead got %v", err)
	}
}

// TestEndToEnd checks the end-to-end functionality of the review tool.
func TestEndToEnd(t *testing.T) {
	workingDir, repoPath, _, gerritPath := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	files := []string{"file1", "file2", "file3"}
	if err := commitFiles(files); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", files, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	review.run()
	expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
	assertFilesPushedToRef(t, repoPath, gerritPath, expectedRef, files)
}

// TestDirtyBranch checks that the tool correctly handles unstaged and
// untracked changes in a working branch with stashed changes.
func TestDirtyBranch(t *testing.T) {
	workingDir, repoPath, _, gerritPath := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	files := []string{"file1", "file2", "file3"}
	if err := commitFiles(files); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", files, err)
	}
	assertStashSize(t, 0)
	stashedFile, stashedFileContent := "stashed-file", "stashed-file content"
	if err := writeFile(stashedFile, stashedFileContent); err != nil {
		t.Fatalf("writeFile(%v, %v) failed: %v", stashedFile, stashedFileContent, err)
	}
	if err := git.Add(stashedFile); err != nil {
		t.Fatalf("Add(%v) failed: %v", stashedFile, err)
	}
	if _, err := git.Stash(); err != nil {
		t.Fatalf("Stash() failed: %v", err)
	}
	assertStashSize(t, 1)
	modifiedFile, modifiedFileContent := "modified-file", "modified-file content"
	if err := writeFile(modifiedFile, modifiedFileContent); err != nil {
		t.Fatalf("writeFile(%v, %v) failed: %v", modifiedFile, modifiedFileContent, err)
	}
	stagedFile, stagedFileContent := "staged-file", "staged-file content"
	if err := writeFile(stagedFile, stagedFileContent); err != nil {
		t.Fatalf("writeFile(%v, %v) failed: %v", stagedFile, stagedFileContent, err)
	}
	if err := git.Add(stagedFile); err != nil {
		t.Fatalf("Add(%v) failed: %v", stagedFile, err)
	}
	untrackedFile, untrackedFileContent := "untracked-file", "untracked-file content"
	if err := writeFile(untrackedFile, untrackedFileContent); err != nil {
		t.Fatalf("writeFile(%v, %t) failed: %v", untrackedFile, untrackedFileContent, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	review.run()
	expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
	assertFilesPushedToRef(t, repoPath, gerritPath, expectedRef, files)
	assertFilesNotCommitted(t, []string{stagedFile})
	assertFilesNotCommitted(t, []string{untrackedFile})
	assertFileContent(t, modifiedFile, modifiedFileContent)
	assertFileContent(t, stagedFile, stagedFileContent)
	assertFileContent(t, untrackedFile, untrackedFileContent)
	assertStashSize(t, 1)
	if err := git.StashPop(); err != nil {
		t.Fatalf("StashPop() failed: %v", err)
	}
	assertStashSize(t, 0)
	assertFilesNotCommitted(t, []string{stashedFile})
	assertFileContent(t, stashedFile, stashedFileContent)
}

// TestRunInSubdirectory checks that the command will succeed when run from
// within a subdirectory of a branch that does not exist on master branch, and
// will return the user to the subdirectory after completion.
func TestRunInSubdirectory(t *testing.T) {
	workingDir, repoPath, _, gerritPath := setup(t, true)
	defer teardown(t, workingDir)
	branch := "my-branch"
	if err := git.CreateAndCheckoutBranch(branch); err != nil {
		t.Fatalf("CreateAndCheckoutBranch(%v) failed: %v", branch, err)
	}
	subdir := "sub/directory"
	subdirPerms := os.FileMode(0744)
	if err := os.MkdirAll(subdir, subdirPerms); err != nil {
		t.Fatalf("os.MkdirAll(%v, %v) failed: %v", subdir, subdirPerms, err)
	}
	files := []string{path.Join(subdir, "file1")}
	if err := commitFiles(files); err != nil {
		t.Fatalf("commitFiles(%v) failed: %v", files, err)
	}
	if err := os.Chdir(subdir); err != nil {
		t.Fatalf("os.Chdir(%v) failed: %v", subdir, err)
	}
	ctx := util.NewContext(true, os.Stdout, os.Stderr)
	draft, edit, repo, reviewers, ccs := false, false, gerritPath, "", ""
	review, err := NewReview(ctx, draft, edit, repo, reviewers, ccs)
	if err != nil {
		t.Fatalf("%v", err)
	}
	review.run()
	path := path.Join(repoPath, subdir)
	expected, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%v) failed: %v", path, err)
	}
	workingDir, err = os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() failed: %v", err)
	}
	got, err := filepath.EvalSymlinks(workingDir)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%v) failed: %v", workingDir, err)
	}
	if expected != got {
		t.Fatalf("Unexpected working direcotry, expected %v, got %v", expected, got)
	}
	expectedRef := gerrit.Reference(draft, reviewers, ccs, branch)
	assertFilesPushedToRef(t, repoPath, gerritPath, expectedRef, files)
}