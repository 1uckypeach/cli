// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"fmt"

	"github.com/larksuite/cli/shortcuts/common"
)

const aliasFlagNoticeAnnotation = "lark-cli.im/alias-notice-emitted"

// aliasFlagValue handles a renamed string flag whose old name is kept as a
// hidden alias. It is only for flags with identical semantics and value
// domains; value-aware compatibility such as +chat-search --types stays in
// that command's validation. It returns (oldValue, true) only when the old
// flag was explicitly used and the new one was not. The canonical flag wins
// when both are present. A note is emitted once per invocation when the alias
// is used.
func aliasFlagValue(rt *common.RuntimeContext, oldName, newName string) (string, bool) {
	if rt.Changed(oldName) && !rt.Changed(newName) {
		emitAliasFlagNote(rt, oldName, newName)
		return rt.Str(oldName), true
	}
	return "", false
}

// aliasIntFlagValue is the typed equivalent of aliasFlagValue for int flags.
func aliasIntFlagValue(rt *common.RuntimeContext, oldName, newName string) (int, bool) {
	if rt.Changed(oldName) && !rt.Changed(newName) {
		emitAliasFlagNote(rt, oldName, newName)
		return rt.Int(oldName), true
	}
	return 0, false
}

func emitAliasFlagNote(rt *common.RuntimeContext, oldName, newName string) {
	if rt == nil || rt.Cmd == nil || rt.Factory == nil || rt.Factory.IOStreams == nil || rt.Factory.IOStreams.ErrOut == nil {
		return
	}
	flag := rt.Cmd.Flags().Lookup(oldName)
	if flag == nil {
		return
	}
	if len(flag.Annotations[aliasFlagNoticeAnnotation]) > 0 {
		return
	}
	if flag.Annotations == nil {
		flag.Annotations = make(map[string][]string)
	}
	flag.Annotations[aliasFlagNoticeAnnotation] = []string{newName}
	fmt.Fprintf(rt.Factory.IOStreams.ErrOut, "note: --%s is an alias for --%s\n", oldName, newName)
}
