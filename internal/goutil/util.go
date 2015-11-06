// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package goutil provides Go wrappers around the Go command-line
// tool.
package goutil

import (
	"bytes"
	"fmt"
	"strings"

	"v.io/jiri/tool"
)

// List inputs a list of Go package expressions and returns a list of
// Go packages that can be found in the GOPATH and match any of the
// expressions. The implementation invokes 'go list' internally with
// jiriArgs as arguments to the jiri-go subcommand.
func List(ctx *tool.Context, jiriArgs []string, pkgs ...string) ([]string, error) {
	return list(ctx, jiriArgs, "{{.ImportPath}}", pkgs...)
}

// ListDirs inputs a list of Go package expressions and returns a list of
// directories that match the expressions.  The implementation invokes 'go list'
// internally with jiriArgs as arguments to the jiri-go subcommand.
func ListDirs(ctx *tool.Context, jiriArgs []string, pkgs ...string) ([]string, error) {
	return list(ctx, jiriArgs, "{{.Dir}}", pkgs...)
}

func list(ctx *tool.Context, jiriArgs []string, format string, pkgs ...string) ([]string, error) {
	args := append([]string{"go"}, jiriArgs...)
	args = append(args, "list", "-f="+format)
	args = append(args, pkgs...)
	var out bytes.Buffer
	opts := ctx.Run().Opts()
	opts.Stdout = &out
	opts.Stderr = &out
	if err := ctx.Run().CommandWithOpts(opts, "jiri", args...); err != nil {
		fmt.Fprintln(ctx.Stderr(), out.String())
		return nil, err
	}
	cleanOut := strings.TrimSpace(out.String())
	if cleanOut == "" {
		return nil, nil
	}
	return strings.Split(cleanOut, "\n"), nil
}
