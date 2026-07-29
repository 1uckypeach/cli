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

// Text that visually reorders itself can make a listing row read as something
// other than what it says, so every bidi-control group has to go — not just the
// embedding/override one.
func TestSanitizeIndexDesc_StripsAllBidiControls(t *testing.T) {
	for _, r := range []rune{
		0x202a, 0x202b, 0x202c, 0x202d, 0x202e, // embedding / override
		0x2066, 0x2067, 0x2068, 0x2069, // isolates
		0x061c, // arabic letter mark
		0x200b, 0x200e, 0x200f, 0xfeff,
	} {
		in := "safe" + string(r) + "tail"
		if got := SanitizeIndexDesc(in); got != "safetail" {
			t.Errorf("SanitizeIndexDesc(U+%04X) = %q, want %q", r, got, "safetail")
		}
	}
}

// C1 carries a second set of escape introducers (0x9b is CSI), so folding only
// C0 would leave those usable.
func TestSanitizeIndexDesc_FoldsC1Controls(t *testing.T) {
	for _, r := range []rune{0x80, 0x9b, 0x9f} {
		in := "a" + string(r) + "b"
		if got := SanitizeIndexDesc(in); got != "a b" {
			t.Errorf("SanitizeIndexDesc(U+%04X) = %q, want %q", r, got, "a b")
		}
	}
}
