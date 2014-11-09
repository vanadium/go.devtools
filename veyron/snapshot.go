package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"tools/lib/cmdline"
	"tools/lib/runutil"
	"tools/lib/util"
)

var cmdSnapshot = &cmdline.Command{
	Name:  "snapshot",
	Short: "Manage snapshots of the veyron project",
	Long: `
The "veyron snapshot" command can be used to manage snapshots of the
veyron project. In particular, it can be used to create new snapshots
and to list existing snapshots.

The command-line flag "-remote" determines whether the command
pertains to "local" snapshots that are only stored locally or "remote"
snapshots the are revisioned in the manifest repository.
`,
	Children: []*cmdline.Command{cmdSnapshotCreate, cmdSnapshotList},
}

// cmdSnapshotCreate represents the "create" sub-command of the
// "snapshot" command of the veyron tool.
var cmdSnapshotCreate = &cmdline.Command{
	Run:   runSnapshotCreate,
	Name:  "create",
	Short: "Create a new snapshot of the veyron project",
	Long: `
The "veyron snapshot create <label>" command first checks whether the
veyron tool configuration associates the given label with any
tests. If so, the command checks that all of these tests pass.

Next, the command captures the current state of the veyron project as a
manifest and, depending on the value of the -remote flag, the command
either stores the manifest in the local $VEYRON_ROOT/.snapshots
directory, or in the manifest repository, pushing the change to the
remote repository and thus making it available globally.

Internally, snapshots are organized as follows:

 <snapshot-dir>/
   labels/
     <label1>/
       <label1-snapshot1>
       <label1-snapshot2>
       ...
     <label2>/
       <label2-snapshot1>
       <label2-snapshot2>
       ...
     <label3>/
     ...
   <label1> # a symlink to the latest <label1-snapshot*>
   <label2> # a symlink to the latest <label2-snapshot*>
   ...

NOTE: Unlike the veyron tool commands, the above internal organization
is not an API. It is an implementation and can change without notice.
`,
	ArgsName: "<label>",
	ArgsLong: "<label> is the snapshot label.",
}

func runSnapshotCreate(command *cmdline.Command, args []string) error {
	if len(args) != 1 {
		return command.UsageErrorf("unexpected number of arguments")
	}
	ctx, label := util.NewContextFromCommand(command, verboseFlag), args[0]

	if !remoteFlag {
		if err := checkSnapshotDir(ctx); err != nil {
			return err
		}
	}

	// Run the tests associated with the given label. The creation
	// of "remote" snapshots requires that the label exists in the
	// veyron tool configuration, while creationg "local"
	// snapshots does not have that requirement.
	if err := runTests(ctx, label); err != nil {
		return err
	}

	snapshotDir, err := getSnapshotDir()
	if err != nil {
		return err
	}
	snapshotFile := filepath.Join(snapshotDir, "labels", label, time.Now().Format(time.RFC3339))
	// Either atomically create a new snapshot that captures the
	// state of the veyron project and push the changes to the
	// remote repository (if applicable), or fail with no effect.
	createFn := func() error {
		revision, err := ctx.Git().LatestCommitID()
		if err != nil {
			return err
		}
		if err := createSnapshot(ctx, snapshotDir, snapshotFile, label); err != nil {
			// Clean up on all errors.
			ctx.Git().Reset(revision)
			ctx.Git().RemoveUntrackedFiles()
			return err
		}
		return nil
	}

	// Execute the above function in the snapshot directory.
	project := util.Project{
		Path:     snapshotDir,
		Protocol: "git",
		Revision: "HEAD",
	}
	if err := util.ApplyToLocalMaster(ctx, project, createFn); err != nil {
		return err
	}
	return nil
}

// checkSnapshotDir makes sure that he local snapshot directory exists
// and is initialized properly.
func checkSnapshotDir(ctx *util.Context) error {
	snapshotDir, err := util.LocalSnapshotDir()
	if err != nil {
		return err
	}
	if _, err := os.Stat(snapshotDir); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		createFn := func() error {
			if err := ctx.Run().Function(runutil.MkdirAll(snapshotDir, 0755)); err != nil {
				return err
			}
			if err := ctx.Git().Init(snapshotDir); err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			defer os.Chdir(cwd)
			if err := ctx.Run().Function(runutil.Chdir(snapshotDir)); err != nil {
				return err
			}
			if err := ctx.Git().Commit(); err != nil {
				return err
			}
			return nil
		}
		if err := createFn(); err != nil {
			ctx.Run().Function(runutil.RemoveAll(snapshotDir))
			return err
		}
	}
	return nil
}

func createSnapshot(ctx *util.Context, snapshotDir, snapshotFile, label string) error {
	// Create a snapshot that encodes the current state of master
	// branches for all local projects.
	if err := util.CreateSnapshot(ctx, snapshotFile); err != nil {
		return err
	}

	// Update the symlink for this snapshot label to point to the
	// latest snapshot.
	symlink := filepath.Join(snapshotDir, label)
	newSymlink := symlink + ".new"
	if err := ctx.Run().Function(runutil.RemoveAll(newSymlink)); err != nil {
		return err
	}
	if err := ctx.Run().Function(runutil.Symlink(snapshotFile, newSymlink)); err != nil {
		return err
	}
	if err := ctx.Run().Function(runutil.Rename(newSymlink, symlink)); err != nil {
		return err
	}

	// Revision the changes.
	if err := revisionChanges(ctx, snapshotDir, snapshotFile, label); err != nil {
		return err
	}
	return nil
}

// getSnapshotDir returns the path to the snapshot directory,
// respecting the value of the "-remote" command-line flag.
func getSnapshotDir() (string, error) {
	if remoteFlag {
		snapshotDir, err := util.RemoteManifestDir()
		if err != nil {
			return "", err
		}
		return snapshotDir, nil
	}
	snapshotDir, err := util.LocalSnapshotDir()
	if err != nil {
		return "", err
	}
	return snapshotDir, nil
}

// revisionChanges commits changes identified by the given manifest
// file and label to the manifest repository and (if applicable)
// pushes these changes to the remote repository.
func revisionChanges(ctx *util.Context, snapshotDir, snapshotFile, label string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer os.Chdir(cwd)
	if err := ctx.Run().Function(runutil.Chdir(snapshotDir)); err != nil {
		return err
	}
	if err := ctx.Git().Add(snapshotFile); err != nil {
		return err
	}
	if err := ctx.Git().Add(filepath.Join(snapshotDir, label)); err != nil {
		return err
	}
	name := strings.TrimPrefix(snapshotFile, snapshotDir)
	if err := ctx.Git().CommitWithMessage(fmt.Sprintf("adding snapshot %q for label %q", name, label)); err != nil {
		return err
	}
	if remoteFlag {
		if err := ctx.Git().Push("origin", "master"); err != nil {
			return err
		}
	}
	return nil
}

// runTests runs the tests associated with the given snapshot label.
func runTests(ctx *util.Context, label string) error {
	root, err := util.VeyronRoot()
	if err != nil {
		return err
	}
	config, err := util.VeyronConfig()
	if err != nil {
		return err
	}
	tests, ok := config.TestMap[label]
	if !ok {
		if remoteFlag {
			return fmt.Errorf("no configuration for label %v found", label)
		}
		return nil
	}
	for _, test := range tests {
		testPath := filepath.Join(root, "scripts", "jenkins", test)
		testCmd := exec.Command(testPath)
		testCmd.Stdout = ctx.Stdout()
		testCmd.Stdout = ctx.Stderr()
		if err := testCmd.Run(); err != nil {
			return fmt.Errorf("%v failed: %v", strings.Join(testCmd.Args, " "), err)
		}
	}
	return nil
}

// cmdSnapshotList represents the "list" sub-command of the "snapshot"
// command of the veyron tool.
var cmdSnapshotList = &cmdline.Command{
	Run:   runSnapshotList,
	Name:  "list",
	Short: "List existing snapshots of veyron builds",
	Long: `
The "snapshot list" command lists existing snapshots of the labels
specified as command-line arguments. If no arguments are provided, the
command lists snapshots for all known labels.
`,
	ArgsName: "<label ...>",
	ArgsLong: "<label ...> is a list of snapshot labels.",
}

func runSnapshotList(command *cmdline.Command, args []string) error {
	snapshotDir, err := getSnapshotDir()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		// Identify all known snapshot labels, using a
		// heuristic that looks for all symbolic links <foo>
		// in the snapshot directory that point to a file in
		// the "labels/<foo>" subdirectory of the snapshot
		// directory.
		fileInfoList, err := ioutil.ReadDir(snapshotDir)
		if err != nil {
			return fmt.Errorf("ReadDir(%v) failed: %v", snapshotDir, err)
		}
		for _, fileInfo := range fileInfoList {
			if fileInfo.Mode()&os.ModeSymlink != 0 {
				path := filepath.Join(snapshotDir, fileInfo.Name())
				dst, err := filepath.EvalSymlinks(path)
				if err != nil {
					return fmt.Errorf("EvalSymlinks(%v) failed: %v", path, err)
				}
				if strings.HasSuffix(filepath.Dir(dst), filepath.Join("labels", fileInfo.Name())) {
					args = append(args, fileInfo.Name())
				}
			}
		}
	}

	// Check that all labels exist.
	failed := false
	for _, label := range args {
		labelDir := filepath.Join(snapshotDir, "labels", label)
		if _, err := os.Stat(labelDir); err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			failed = true
			fmt.Fprintf(command.Stderr(), "snapshot label %q not found", label)
		}
	}
	if failed {
		return cmdline.ErrExitCode(2)
	}

	// Print snapshots for all labels.
	sort.Strings(args)
	for _, label := range args {
		// Scan the snapshot directory "labels/<label>" printing
		// all snapshots.
		labelDir := filepath.Join(snapshotDir, "labels", label)
		fileInfoList, err := ioutil.ReadDir(labelDir)
		if err != nil {
			return fmt.Errorf("ReadDir(%v) failed: %v", labelDir, err)
		}
		fmt.Fprintf(command.Stdout(), "snapshots of label %q:\n", label)
		for _, fileInfo := range fileInfoList {
			fmt.Fprintf(command.Stdout(), "  %v\n", fileInfo.Name())
		}
	}
	return nil
}
