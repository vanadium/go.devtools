// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Command oncall implements oncall specific utilities used by Vanadium team.

Usage:
   oncall [flags] <command>

The oncall commands are:
   serve       Serve oncall dashboard data from Google Storage
   help        Display help for commands or topics

The oncall flags are:
 -color=true
   Use color to format output.
 -v=false
   Print verbose output.

The global flags are:
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.

Oncall serve - Serve oncall dashboard data from Google Storage

Serve oncall dashboard data from Google Storage.

Usage:
   oncall serve [flags]

The oncall serve flags are:
 -address=:8000
   Listening address for the server.
 -cache=
   Directory to use for caching files.
 -key=
   The path to the service account's JSON credentials file.
 -static=
   Directory to use for serving static files.

 -color=true
   Use color to format output.
 -v=false
   Print verbose output.

Oncall help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   oncall help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The oncall help flags are:
 -style=compact
   The formatting style for help output:
      compact   - Good for compact cmdline output.
      full      - Good for cmdline output, shows all global flags.
      godoc     - Good for godoc processing.
      shortonly - Only output short description.
   Override the default by setting the CMDLINE_STYLE environment variable.
 -width=<terminal width>
   Format output to this target width in runes, or unlimited if width < 0.
   Defaults to the terminal width if available.  Override the default by setting
   the CMDLINE_WIDTH environment variable.
*/
package main
