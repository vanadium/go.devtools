// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package base

import (
	"flag"
	"fmt"

	"v.io/jiri/jiri"
	"v.io/jiri/profiles"
	"v.io/jiri/profiles/profilesmanager"
	"v.io/jiri/profiles/profilesreader"
	"v.io/x/lib/envvar"
)

const (
	profileName = "base"
)

type versionSpec struct {
	dependencies []struct{ name, version string }
}

func init() {
	m := &Manager{
		versionInfo: profiles.NewVersionInfo(profileName,
			map[string]interface{}{
				"1": &versionSpec{[]struct{ name, version string }{
					{"go", ""},
					{"syncbase", ""}},
				},
				"2": &versionSpec{[]struct{ name, version string }{
					{"go", "1.5.1.1:2738c5e0"},
					{"syncbase", ""}},
				},
				"3": &versionSpec{[]struct{ name, version string }{
					{"go", "1.5.2"},
					{"syncbase", ""}},
				},
				"4": &versionSpec{[]struct{ name, version string }{
					{"go", "1.5.2.1:56093743"},
					{"syncbase", ""}},
				},
			}, "1"),
	}
	profilesmanager.Register(profileName, m)
}

type Manager struct {
	versionInfo *profiles.VersionInfo
	spec        versionSpec
}

func (Manager) Name() string {
	return profileName
}

func (m Manager) String() string {
	return fmt.Sprintf("%s[%s]", profileName, m.versionInfo.Default())
}

func (m Manager) Info() string {
	return `
The base profile is a convenient shorthand for installing the profiles that all
vanadium projects need, this is currently go and syncbase.`
}

func (m Manager) VersionInfo() *profiles.VersionInfo {
	return m.versionInfo
}

func (m *Manager) AddFlags(flags *flag.FlagSet, action profiles.Action) {
}

func (m *Manager) Install(jirix *jiri.X, pdb *profiles.DB, root jiri.RelPath, target profiles.Target) error {
	// Install packages
	if !target.CrossCompiling() && target.OS() == "linux" {
		if err := profiles.InstallPackages(jirix, []string{"libssl-dev"}); err != nil {
			return err
		}
	}
	if err := m.versionInfo.Lookup(target.Version(), &m.spec); err != nil {
		return err
	}
	// Install profiles.
	profileEnvs := [][]string{}
	for _, profile := range m.spec.dependencies {
		dependency := target
		dependency.SetVersion(profile.version)
		if err := profilesmanager.EnsureProfileTargetIsInstalled(jirix, pdb, profile.name, root, dependency); err != nil {
			return err
		}
		installed := pdb.LookupProfileTarget(profile.name, dependency)
		if installed == nil {
			return fmt.Errorf("%s %s should have been installed, but apparently is not", profile.name, dependency)
		}
		profileEnvs = append(profileEnvs, installed.Env.Vars)
	}
	// Merge the environments for go and syncbase and store it in the base profile.
	base := envvar.VarsFromSlice(target.Env.Vars)
	base.Set("GOARCH", target.Arch())
	base.Set("GOOS", target.OS())
	profilesreader.MergeEnv(profilesreader.ProfileMergePolicies(), base, profileEnvs...)
	target.Env.Vars = base.ToSlice()
	pdb.InstallProfile(profileName, root.Symbolic())
	return pdb.AddProfileTarget(profileName, target)
}

func (m *Manager) Uninstall(jirix *jiri.X, pdb *profiles.DB, root jiri.RelPath, target profiles.Target) error {
	if err := m.versionInfo.Lookup(target.Version(), &m.spec); err != nil {
		return err
	}
	for _, profile := range m.spec.dependencies {
		dependency := target
		dependency.SetVersion(profile.version)
		if err := profilesmanager.EnsureProfileTargetIsUninstalled(jirix, pdb, profile.name, root, dependency); err != nil {
			return err
		}
	}
	pdb.RemoveProfileTarget(profileName, target)
	return nil
}
