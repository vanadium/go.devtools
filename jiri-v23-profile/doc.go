// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This file was auto-generated via go generate.
// DO NOT UPDATE MANUALLY

/*
Profiles provide a means of managing software dependencies that can be built
natively as well as being cross compiled. A profile generally manages a suite of
related software components that are required for a particular application (e.g.
for android development).

Each profile can be in one of three states: absent, up-to-date, or out-of-date.
The subcommands of the profile command realize the following transitions:

  install:   absent => up-to-date
  update:    out-of-date => up-to-date
  uninstall: up-to-date or out-of-date => absent

A profile can simultaneously have multiple versions, one of which is configured
as the default. A profile installation is out of date if the installed versions
are older than the current default. Updating that profile will install the
default version which will then be used by default. Newer versions than the
default may be installed and used via appropriate command line flags.

To enable cross-compilation, a profile can be installed for multiple targets. If
a profile supports multiple targets the above state transitions are applied on a
profile + target basis.

Usage:
   jiri v23-profile [flags] <command>

The jiri v23-profile commands are:
   install     Install the given profiles
   list        List available or installed profiles
   env         Display profile environment variables
   uninstall   Uninstall the given profiles
   update      Install the latest default version of the given profiles
   recreate    Display a list of commands that will recreate the currently
               installed profiles
   help        Display help for commands or topics

The jiri v23-profile flags are:
 -color=true
   Use color to format output.
 -n=false
   Show what commands will run but do not execute them.
 -v=false
   Print verbose output.

The global flags are:
 -metadata=<just specify -metadata to activate>
   Displays metadata for the program and exits.
 -time=false
   Dump timing information to stderr before exiting the program.

Jiri v23-profile install - Install the given profiles

Install the given profiles.

Usage:
   jiri v23-profile install [flags] <profiles>

<profiles> is a list of profiles to install.

The jiri v23-profile install flags are:
 -env=
   specifcy an environment variable in the form: <var>=[<val>],...
 -go.install-dir=
   installation directory for go profile builds.
 -go.sysroot=
   sysroot for cross compiling to the currently specified target
 -manifest=$JIRI_ROOT//.jiri_v23_profiles
   specify the profiles XML manifest filename.
 -target=<runtime.GOARCH>-<runtime.GOOS>
   specifies a profile target in the following form:
   <arch>-<os>[@<version>]|<tag>[@version]|<tag>=<arch>-<val>[@<version>]

Jiri v23-profile list - List available or installed profiles

List available or installed profiles.

Usage:
   jiri v23-profile list [flags] [<profiles>]

<profiles> is a list of profiles to list, defaulting to all profiles if none are
specifically requested.

The jiri v23-profile list flags are:
 -available=false
   print the list of available profiles
 -manifest=$JIRI_ROOT//.jiri_v23_profiles
   specify the profiles XML manifest filename.
 -show-manifest=false
   print out the manifest file
 -v=false
   print more detailed information

Jiri v23-profile env - Display profile environment variables

List profile specific and target specific environment variables. If the
requested environment variable name ends in = then only the value will be
printed, otherwise both name and value are printed, i.e. GOPATH="foo" vs just
"foo".

If no environment variable names are requested then all will be printed in
<name>=<val> format.

Usage:
   jiri v23-profile env [flags] [<environment variable names>]

[<environment variable names>] is an optional list of environment variables to
display

The jiri v23-profile env flags are:
 -manifest=$JIRI_ROOT//.jiri_v23_profiles
   specify the profiles XML manifest filename.
 -profile=
   the profile whose environment is to be displayed
 -target=<runtime.GOARCH>-<runtime.GOOS>
   specifies a profile target in the following form:
   <arch>-<os>[@<version>]|<tag>[@version]|<tag>=<arch>-<val>[@<version>]

Jiri v23-profile uninstall - Uninstall the given profiles

Uninstall the given profiles.

Usage:
   jiri v23-profile uninstall [flags] <profiles>

<profiles> is a list of profiles to uninstall.

The jiri v23-profile uninstall flags are:
 -all-targets=false
   apply to all targets for the specified profile(s)
 -go.install-dir=
   installation directory for go profile builds.
 -go.sysroot=
   sysroot for cross compiling to the currently specified target
 -manifest=$JIRI_ROOT//.jiri_v23_profiles
   specify the profiles XML manifest filename.
 -target=<runtime.GOARCH>-<runtime.GOOS>
   specifies a profile target in the following form:
   <arch>-<os>[@<version>]|<tag>[@version]|<tag>=<arch>-<val>[@<version>]

Jiri v23-profile update - Install the latest default version of the given profiles

Install the latest default version of the given profiles.

Usage:
   jiri v23-profile update [flags] <profiles>

<profiles> is a list of profiles to update, if omitted all profiles are updated.

The jiri v23-profile update flags are:
 -manifest=$JIRI_ROOT//.jiri_v23_profiles
   specify the profiles XML manifest filename.
 -v=false
   print more detailed information

Jiri v23-profile recreate - Display a list of commands that will recreate the currently installed profiles

Display a list of commands that will recreate the currently installed profiles.

Usage:
   jiri v23-profile recreate [flags] <profiles>

<profiles> is a list of profiles to be recreated, if omitted commands to
recreate all profiles are displayed.

The jiri v23-profile recreate flags are:
 -manifest=$JIRI_ROOT//.jiri_v23_profiles
   specify the profiles XML manifest filename.

Jiri v23-profile help - Display help for commands or topics

Help with no args displays the usage of the parent command.

Help with args displays the usage of the specified sub-command or help topic.

"help ..." recursively displays help for all commands and topics.

Usage:
   jiri v23-profile help [flags] [command/topic ...]

[command/topic ...] optionally identifies a specific sub-command or help topic.

The jiri v23-profile help flags are:
 -style=compact
   The formatting style for help output:
      compact - Good for compact cmdline output.
      full    - Good for cmdline output, shows all global flags.
      godoc   - Good for godoc processing.
   Override the default by setting the CMDLINE_STYLE environment variable.
 -width=<terminal width>
   Format output to this target width in runes, or unlimited if width < 0.
   Defaults to the terminal width if available.  Override the default by setting
   the CMDLINE_WIDTH environment variable.
*/
package main
