// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package i18n

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		in     string
		want   Lang
		wantOK bool
	}{
		{"zh", LangZhCN, true},    // short code
		{"zh_cn", LangZhCN, true}, // canonical locale
		{"en", LangEnUS, true},    // short code
		{"en_us", LangEnUS, true}, // canonical locale
		{"ja", LangJaJP, true},    // short code
		{"pt", LangPtBR, true},    // pt → pt_br, not pt_pt
		{"ms", LangMsMY, true},    // ms → ms_my
		{"", "", false},           // unset
		{"ZH", "", false},         // case-sensitive
		{"zh-CN", "", false},      // hyphen form not accepted
		{"zh_CN", "", false},      // case-sensitive region
		{"ar", "", false},         // not in the supported set
		{"xx", "", false},         // unknown
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, ok := Parse(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("Parse(%q) = (%q, %v), want (%q, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestBase(t *testing.T) {
	tests := []struct {
		lang Lang
		want string
	}{
		{LangEnUS, "en"},
		{LangZhCN, "zh"},
		{LangJaJP, "ja"},
		{Lang("en"), "en"}, // legacy short value
		{Lang("zh"), "zh"},
		{Lang(""), ""},        // unset
		{Lang("garbage"), ""}, // unknown
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			if got := tt.lang.Base(); got != tt.want {
				t.Errorf("Lang(%q).Base() = %q, want %q", tt.lang, got, tt.want)
			}
		})
	}
}

func TestCodes(t *testing.T) {
	codes := Codes()
	if len(codes) != 14 {
		t.Fatalf("len(Codes()) = %d, want 14", len(codes))
	}
	if codes[0] != "zh_cn" {
		t.Errorf("Codes()[0] = %q, want %q (catalog order)", codes[0], "zh_cn")
	}
	// Every code must round-trip through Parse to itself (canonical).
	for _, c := range codes {
		if got, ok := Parse(c); !ok || string(got) != c {
			t.Errorf("Parse(%q) = (%q, %v), want (%q, true)", c, got, ok, c)
		}
	}
}

func TestCodesWithShort(t *testing.T) {
	got := CodesWithShort()
	if want := "zh_cn (zh), en_us (en), ja_jp (ja)"; !strings.HasPrefix(got, want) {
		t.Errorf("CodesWithShort() = %q, want prefix %q (catalog order)", got, want)
	}
	// Every catalog entry must be listed with both spellings a user may type,
	// so the listing never sends someone hunting for a value it omitted.
	for _, e := range catalog {
		entry := string(e.Code) + " (" + e.Short + ")"
		if !strings.Contains(got, entry) {
			t.Errorf("CodesWithShort() = %q, missing %q", got, entry)
		}
	}
	if n := strings.Count(got, ", "); n != len(catalog)-1 {
		t.Errorf("CodesWithShort() has %d separators, want %d", n, len(catalog)-1)
	}
}

func TestUsesEnglishUI(t *testing.T) {
	// Only two TUI bundles exist. zh_cn — and anything that expresses no
	// usable preference — renders Chinese; every other supported locale
	// renders English.
	chinese := []Lang{
		LangZhCN, "zh", // Chinese, canonical and short
		"",        // unset
		"unknown", // not in the catalog
		"ZH",      // wrong case: find() is case-sensitive
		"en_US",   // wrong case for a real locale
	}
	for _, l := range chinese {
		if l.UsesEnglishUI() {
			t.Errorf("UsesEnglishUI(%q) = true, want false", l)
		}
	}

	english := []Lang{
		LangEnUS, "en",
		LangJaJP, LangKoKR, LangFrFR, LangDeDE, LangEsES, LangItIT,
		LangRuRU, LangPtBR, LangThTH, LangViVN, LangIdID, LangMsMY,
		"ja", // short code for a non-English locale
	}
	for _, l := range english {
		if !l.UsesEnglishUI() {
			t.Errorf("UsesEnglishUI(%q) = false, want true", l)
		}
	}
}

func TestUsesEnglishUI_CoversEveryCatalogEntry(t *testing.T) {
	// Guard against a new locale being added to the catalog without deciding
	// which bundle it renders in.
	for _, code := range Codes() {
		l := Lang(code)
		want := code != string(LangZhCN)
		if got := l.UsesEnglishUI(); got != want {
			t.Errorf("UsesEnglishUI(%q) = %v, want %v", code, got, want)
		}
	}
}
