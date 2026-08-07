// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"github.com/larksuite/cli/internal/core"
	"github.com/spf13/pflag"
)

// GlobalOptions are the root-level flags shared by bootstrap parsing and the
// actual Cobra command tree. Profile is the parsed --profile value; HideProfile
// is a build-time policy — when true, --profile stays parseable but is marked
// hidden from help and shell completion.
type GlobalOptions struct {
	Profile     string
	HideProfile bool
}

// RegisterGlobalFlags registers the root-level persistent flags on fs and
// applies any visibility policy encoded in opts. Pure function: no disk,
// network, or environment reads — the caller decides HideProfile.
func RegisterGlobalFlags(fs *pflag.FlagSet, opts *GlobalOptions) {
	fs.StringVar(&opts.Profile, "profile", "", "use a specific profile")
	if opts.HideProfile {
		_ = fs.MarkHidden("profile")
	}
}

// registeredGlobalFlagArities returns the root flag names and whether each flag
// requires a separate value. Deriving this information from RegisterGlobalFlags
// keeps bootstrap parsing, Cobra, and pre-assembly argv routing in lockstep.
func registeredGlobalFlagArities() (map[string]bool, map[string]bool) {
	fs := pflag.NewFlagSet("global-flag-arities", pflag.ContinueOnError)
	RegisterGlobalFlags(fs, &GlobalOptions{})

	long := make(map[string]bool)
	short := make(map[string]bool)
	fs.VisitAll(func(flag *pflag.Flag) {
		requiresValue := flag.NoOptDefVal == ""
		long[flag.Name] = requiresValue
		if flag.Shorthand != "" {
			short[flag.Shorthand] = requiresValue
		}
	})
	return long, short
}

// isSingleAppMode reports whether the on-disk config has at most one app.
// Missing configs are treated as single-app since --profile is meaningless
// until at least two profiles exist. Intended for the Execute entry point —
// buildInternal must not call this directly to stay state-free.
func isSingleAppMode() bool {
	raw, err := core.LoadMultiAppConfig()
	if err != nil || raw == nil {
		return true
	}
	return len(raw.Apps) <= 1
}
