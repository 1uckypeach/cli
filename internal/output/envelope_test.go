// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package output

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMetaPaginationFieldsOmitempty(t *testing.T) {
	// 有分页 → 出现 has_more/page_token
	b, _ := json.Marshal(Meta{Count: 2, HasMore: true, PageToken: "list:abc"})
	s := string(b)
	if !strings.Contains(s, `"has_more":true`) || !strings.Contains(s, `"page_token":"list:abc"`) {
		t.Fatalf("expected pagination fields, got %s", s)
	}
	// 零值 → omitempty 不出现
	b2, _ := json.Marshal(Meta{Count: 1})
	if strings.Contains(string(b2), "has_more") || strings.Contains(string(b2), "page_token") {
		t.Fatalf("zero pagination must be omitted, got %s", b2)
	}
}

// count 无 omitempty：空结果也显式序列化 count:0，保证列表信封的稳定契约
// （调用方可稳定读取 .meta.count，无需区分“缺失”与“0”）。
func TestMetaCountAlwaysSerialized(t *testing.T) {
	b, _ := json.Marshal(Meta{Count: 0})
	if !strings.Contains(string(b), `"count":0`) {
		t.Fatalf("empty meta must serialize count:0, got %s", b)
	}
}
