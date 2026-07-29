// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package schema

import "testing"

func TestFirstSentence(t *testing.T) {
	cases := []struct{ in, want string }{
		// Upstream descriptions are shaped "<summary>。Identity: <notes>". The
		// identity notes are already exposed as a separate field, so keeping the
		// tail here would only repeat them.
		{"将用户或机器人拉入群聊。Identity: supports `user` and `bot`", "将用户或机器人拉入群聊"},
		{"列出邮件", "列出邮件"},
		{"first line\nsecond line", "first line"},
		{"", ""},
	}
	for _, c := range cases {
		if got := FirstSentence(c.in); got != c.want {
			t.Errorf("FirstSentence(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeIndexDesc_StripsControlAndZeroWidth(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a\x1b[31mred\x1b[0m", "ared"},      // ANSI escapes stripped
		{"tab\there", "tab here"},            // control chars folded to a space
		{"zero​width", "zerowidth"},          // zero-width space removed
		{"line\nbreak", "line break"},        // newline folded, single line out
		{"trail  \t spaces", "trail spaces"}, // space runs collapsed
		{"", ""},
	}
	for _, c := range cases {
		if got := SanitizeIndexDesc(c.in); got != c.want {
			t.Errorf("SanitizeIndexDesc(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
