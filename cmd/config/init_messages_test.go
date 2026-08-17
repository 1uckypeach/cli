// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"testing"

	"github.com/larksuite/cli/internal/i18n"
)

func TestGetInitMsg_Zh(t *testing.T) {
	msg := getInitMsg("zh")
	if msg != initMsgZh {
		t.Error("expected zh message set")
	}
	if msg.SelectAction != "选择操作" {
		t.Errorf("unexpected SelectAction: %s", msg.SelectAction)
	}
}

func TestGetInitMsg_En(t *testing.T) {
	msg := getInitMsg("en")
	if msg != initMsgEn {
		t.Error("expected en message set")
	}
	if msg.SelectAction != "Select action" {
		t.Errorf("unexpected SelectAction: %s", msg.SelectAction)
	}
}

func TestGetInitMsg_DefaultsToZh(t *testing.T) {
	for _, lang := range []i18n.Lang{"", "unknown", "xyz", "invalid"} {
		msg := getInitMsg(lang)
		if msg != initMsgZh {
			t.Errorf("getInitMsg(%q) should default to zh", lang)
		}
	}
}

func TestInitMsgZh_AllFieldsNonEmpty(t *testing.T) {
	assertAllFieldsNonEmpty(t, initMsgZh, "zh")
}

func TestInitMsgEn_AllFieldsNonEmpty(t *testing.T) {
	assertAllFieldsNonEmpty(t, initMsgEn, "en")
}

func assertAllFieldsNonEmpty(t *testing.T, msg *initMsg, label string) {
	t.Helper()
	fields := map[string]string{
		"SelectAction":         msg.SelectAction,
		"CreateNewApp":         msg.CreateNewApp,
		"ConfigExistingApp":    msg.ConfigExistingApp,
		"Platform":             msg.Platform,
		"SelectPlatform":       msg.SelectPlatform,
		"Feishu":               msg.Feishu,
		"ScanQRCode":           msg.ScanQRCode,
		"ScanOrOpenLink":       msg.ScanOrOpenLink,
		"WaitingForScan":       msg.WaitingForScan,
		"OpenLinkNonTTY":       msg.OpenLinkNonTTY,
		"WaitingForScanNonTTY": msg.WaitingForScanNonTTY,
		"DetectedLarkTenant":   msg.DetectedLarkTenant,
		"AppCreated":           msg.AppCreated,
		"ConfigSaved":          msg.ConfigSaved,
		"LangPreferenceSet":    msg.LangPreferenceSet,
	}
	for name, val := range fields {
		if val == "" {
			t.Errorf("%s.%s is empty", label, name)
		}
	}
}

func TestInitMsg_FormatStrings(t *testing.T) {
	for _, lang := range []i18n.Lang{i18n.LangZhCN, i18n.LangEnUS} {
		msg := getInitMsg(lang)
		// AppCreated and ConfigSaved should contain %s for App ID
		got := fmt.Sprintf(msg.AppCreated, "cli_test123")
		if got == msg.AppCreated {
			t.Errorf("%s AppCreated has no format verb", lang)
		}
		got = fmt.Sprintf(msg.ConfigSaved, "cli_test123")
		if got == msg.ConfigSaved {
			t.Errorf("%s ConfigSaved has no format verb", lang)
		}
	}
}

func TestGetInitMsg_BilingualCollapse(t *testing.T) {
	// The TUI is bilingual (zh + en). zh_cn renders Chinese; so do values that
	// express no usable preference (unset, unrecognized). Every other supported
	// locale renders English.
	tests := []struct {
		lang       i18n.Lang
		shouldBeEn bool
	}{
		{i18n.LangZhCN, false},
		{i18n.LangEnUS, true},
		{"en", true},          // legacy short value
		{i18n.LangJaJP, true}, // no ja bundle → English is the closer read
		{"fr_fr", true},       // same for every other supported locale
		{"invalid", false},    // unrecognized → no preference expressed
		{"", false},           // unset → no preference expressed
	}

	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			msg := getInitMsg(tt.lang)
			if msg == nil {
				t.Fatal("getInitMsg returned nil")
			}
			want := initMsgZh
			if tt.shouldBeEn {
				want = initMsgEn
			}
			if msg != want {
				t.Errorf("getInitMsg(%q) returned wrong struct", tt.lang)
			}
		})
	}
}

func TestPickerCanExpress(t *testing.T) {
	// The picker only offers 中文 and English. Unset has nothing to lose, and
	// zh_cn/en_us are its own options — those three can round-trip.
	for _, l := range []i18n.Lang{"", i18n.LangZhCN, i18n.LangEnUS} {
		if !pickerCanExpress(l) {
			t.Errorf("pickerCanExpress(%q) = false, want true", l)
		}
	}
	// Everything else would be silently rewritten to zh_cn or en_us by a bare
	// Enter, so those runs must skip the picker instead of destroying it.
	for _, l := range []i18n.Lang{
		i18n.LangJaJP, i18n.LangKoKR, i18n.LangFrFR, i18n.LangDeDE, i18n.LangEsES,
		i18n.LangItIT, i18n.LangRuRU, i18n.LangPtBR, i18n.LangThTH, i18n.LangViVN,
		i18n.LangIdID, i18n.LangMsMY,
	} {
		if pickerCanExpress(l) {
			t.Errorf("pickerCanExpress(%q) = true, want false", l)
		}
	}
	// --lang cannot produce these (ParseLangFlag canonicalizes before UILang is
	// resolved), but a hand-edited config.json can: the load path stores Lang
	// verbatim. The guard must reject them rather than assume canonical input.
	for _, l := range []i18n.Lang{"zh", "en", "ZH", "en_US"} {
		if pickerCanExpress(l) {
			t.Errorf("pickerCanExpress(%q) = true, want false", l)
		}
	}
}
