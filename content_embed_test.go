// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/vfs"
)

var workplaceContractPaths = []string{
	"skills/lark-doc/references/genres/memo-brief.md",
	"skills/lark-doc/references/genres/weekly-report.md",
	"skills/lark-doc/references/genres/proposal.md",
	"skills/lark-doc/references/genres/execution-plan.md",
	"skills/lark-doc/references/genres/formal-doc.md",
	"skills/lark-doc/references/genres/official-redhead.md",
	"skills/lark-doc/references/genres/meeting-minutes.md",
	"skills/lark-doc/references/genres/retrospective.md",
	"skills/lark-doc/references/genres/prd.md",
	"skills/lark-doc/references/genres/technical-doc.md",
	"skills/lark-doc/references/genres/sop-tutorial.md",
}

var reportContractPaths = []string{
	"skills/lark-doc/references/genres/research-report.md",
	"skills/lark-doc/references/genres/data-report.md",
	"skills/lark-doc/references/genres/white-paper.md",
	"skills/lark-doc/references/genres/business-analysis.md",
}

var platformContractPaths = []string{
	xiaohongshuContractPath,
	wechatContractPath,
}

var standaloneContractPaths = []string{
	"skills/lark-doc/references/genres/route-knowledge.md",
	"skills/lark-doc/references/genres/route-media.md",
	"skills/lark-doc/references/genres/route-opinion.md",
	"skills/lark-doc/references/genres/route-consumer.md",
	"skills/lark-doc/references/genres/route-marketing.md",
	"skills/lark-doc/references/genres/route-personal-brand.md",
	"skills/lark-doc/references/genres/route-creative.md",
}

var routerLeaves = map[string][]string{
	"skills/lark-doc/references/genres/route-workplace.md": {
		"memo-brief.md",
		"weekly-report.md",
		"proposal.md",
		"execution-plan.md",
		"formal-doc.md",
		"official-redhead.md",
		"meeting-minutes.md",
		"retrospective.md",
		"prd.md",
		"technical-doc.md",
		"sop-tutorial.md",
	},
	"skills/lark-doc/references/genres/route-report.md": {
		"research-report.md",
		"data-report.md",
		"white-paper.md",
		"business-analysis.md",
	},
	platformRouterPath: {
		"xiaohongshu.md",
		"wechat.md",
	},
}

const (
	officialDocumentContractPath = "skills/lark-doc/references/genres/official-redhead.md"
	platformRouterPath           = "skills/lark-doc/references/genres/route-platform.md"
	xiaohongshuContractPath      = "skills/lark-doc/references/genres/xiaohongshu.md"
	wechatContractPath           = "skills/lark-doc/references/genres/wechat.md"
)

var blockRuleRowsByContract = map[string][]string{
	officialDocumentContractPath: {"允许 block", "少用 block", "禁止 block"},
	xiaohongshuContractPath:      {"禁止 block"},
	wechatContractPath:           {"禁止 block"},
}

var inlineCodePattern = regexp.MustCompile("`([^`\\n]+)`")
var blockTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
var bareASCIINamePattern = regexp.MustCompile(`[A-Za-z][A-Za-z0-9_-]*`)
var markdownLinkPattern = regexp.MustCompile(`\]\(([^)]+\.md)\)`)

func TestEmbeddedGenreInventoryAndRoles(t *testing.T) {
	classified := allGenrePaths()
	expected := map[string]bool{}
	for _, filename := range classified {
		expected[filename] = true
	}
	if len(expected) != len(classified) {
		t.Fatalf("genre role classification contains duplicate paths: %v", classified)
	}

	actual := map[string]bool{}
	err := fs.WalkDir(embeddedContentFS, "skills/lark-doc/references/genres", func(filename string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(filename, ".md") {
			actual[filename] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk embedded genres: %v", err)
	}

	if diff := setDifference(expected, actual); len(diff) > 0 {
		t.Errorf("classified genre files missing from embed: %v", diff)
	}
	if diff := setDifference(actual, expected); len(diff) > 0 {
		t.Errorf("embedded genre files missing role classification: %v", diff)
	}
}

func TestEmbeddedGenreContractSchema(t *testing.T) {
	legacySections := []string{
		"## Voice",
		"## Presentation bounds",
		"## Failure modes",
		"## Final check",
	}

	for _, filename := range hardRuleTableContractPaths() {
		filename := filename
		t.Run(path.Base(filename), func(t *testing.T) {
			contract := readEmbeddedContent(t, filename)
			ruleCells, ruleOrder := parseContractRuleTable(t, contract)
			for _, section := range legacySections {
				if strings.Contains(contract, section) {
					t.Errorf("contract still contains legacy section %q", section)
				}
			}

			blockRuleRows := blockRuleRowsByContract[filename]
			wantOrder := []string{"presentation_mode / 表达模式"}
			wantOrder = append(wantOrder, blockRuleRows...)
			wantOrder = append(wantOrder, "内容逻辑", "事实 / 边界", "错误")
			if len(blockRuleRows) > 0 {
				assertContractBlockNamesComeFromProfile(t, ruleCells, blockRuleRows)
			}
			if filename == officialDocumentContractPath {
				assertContractBlockLifecycle(t, ruleCells)
			}
			if !reflect.DeepEqual(ruleOrder, wantOrder) {
				t.Errorf("hard-rule rows = %v, want exactly %v", ruleOrder, wantOrder)
			}
		})
	}
}

func TestEmbeddedLeafRoutersAreTablesOnlyAndResolveEveryLeaf(t *testing.T) {
	for routerPath, expectedLeaves := range routerLeaves {
		routerPath, expectedLeaves := routerPath, expectedLeaves
		t.Run(path.Base(routerPath), func(t *testing.T) {
			router := readEmbeddedContent(t, routerPath)
			lines := splitMarkdownLines(router)
			if len(lines) != 6+len(expectedLeaves) {
				t.Fatalf("leaf router must be exactly title, one principle, and one table; got %d lines", len(lines))
			}
			if !strings.HasPrefix(lines[0], "# Genre Router:") || lines[1] != "" || lines[2] == "" || strings.HasPrefix(lines[2], "#") || strings.HasPrefix(lines[2], "|") || lines[3] != "" {
				t.Fatal("leaf router must start with a title and exactly one principle paragraph")
			}
			if lines[4] != "| 读者任务 / 关键词、强信号与排除 | Leaf |" || lines[5] != "|-|-|" {
				t.Fatal("leaf router must use the canonical two-column route table")
			}

			foundLeaves := make([]string, 0, len(expectedLeaves))
			for _, row := range lines[6:] {
				columns, ok := markdownTableRow(row)
				if !ok || len(columns) != 2 || columns[0] == "" || columns[1] == "" {
					t.Fatalf("each route row must contain a non-empty routing rule and leaf: %q", row)
				}
				if !strings.Contains(columns[0], "；") || !containsAnySubstring(columns[0], []string{"走", "排除", "不触发"}) {
					t.Fatalf("each route row must state reader task/signals and a neighboring-genre exclusion: %q", row)
				}
				if markdownLinkPattern.MatchString(columns[0]) {
					t.Fatalf("route semantics column must not contain leaf links: %q", row)
				}
				matches := markdownLinkPattern.FindAllStringSubmatch(columns[1], -1)
				if len(matches) != 1 {
					t.Fatalf("each route row must reference exactly one leaf: %q", row)
				}
				leaf := matches[0][1]
				foundLeaves = append(foundLeaves, leaf)
				readEmbeddedContent(t, path.Join(path.Dir(routerPath), leaf))
			}
			if !reflect.DeepEqual(foundLeaves, expectedLeaves) {
				t.Errorf("router leaves = %v, want exactly %v", foundLeaves, expectedLeaves)
			}
		})
	}
}

func TestSplitMarkdownLinesNormalizesCRLF(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "LF", content: "first\nsecond\n"},
		{name: "CRLF", content: "first\r\nsecond\r\n"},
		{name: "no trailing newline", content: "first\nsecond"},
	}
	want := []string{"first", "second"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := splitMarkdownLines(tt.content); !reflect.DeepEqual(got, want) {
				t.Fatalf("splitMarkdownLines() = %q, want %q", got, want)
			}
		})
	}
}

func TestEmbeddedLarkDocFileExamplesAreCrossShellSafe(t *testing.T) {
	files := []string{
		"skills/lark-doc/references/lark-doc-authoring.md",
		"skills/lark-doc/references/lark-doc-create.md",
		"skills/lark-doc/references/lark-doc-md.md",
		"skills/lark-doc/references/lark-doc-script.md",
	}
	bareFileArgument := regexp.MustCompile(`--(?:content|reference-map)\s+@`)
	for _, filename := range files {
		filename := filename
		t.Run(path.Base(filename), func(t *testing.T) {
			content := readEmbeddedContent(t, filename)
			for _, forbidden := range []string{"mktemp", "$(printf", "<<'EOF'", "cat document |"} {
				if strings.Contains(content, forbidden) {
					t.Errorf("cross-shell docs must not require %q", forbidden)
				}
			}
			if bareFileArgument.MatchString(content) {
				t.Error("@file command arguments must be quoted for PowerShell compatibility")
			}
		})
	}

	for _, filename := range []string{
		"skills/lark-doc/references/lark-doc-create.md",
		"skills/lark-doc/references/lark-doc-update.md",
	} {
		content := readEmbeddedContent(t, filename)
		if strings.Contains(content, "mktemp") {
			t.Errorf("%s must not require a platform-specific temporary-file command", filename)
		}
		for _, required := range []string{"任务独占", "名称唯一", "临时 XML 文件"} {
			if !strings.Contains(content, required) {
				t.Errorf("%s must require %q", filename, required)
			}
		}
	}
}

func TestEmbeddedPlatformContracts(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		title     string
		wants     []string
		forbidden []string
	}{
		{
			name:     "xiaohongshu",
			filename: xiaohongshuContractPath,
			title:    "# Genre Contract: Xiaohongshu Note / 小红书笔记 (`platform.xiaohongshu`)",
			wants: []string{
				"## 笔记主任务",
				"教程 / 攻略 / 知识",
				"体验 / 测评 / 探店",
				"个人经历 / 成长",
				"小红书运营方案走 Workplace",
				"小红书风格偏爱 emoji、图文并茂和清晰轻松的阅读体验",
				"不要求固定收尾动作",
			},
			forbidden: []string{
				"## 飞书稿件格式规则",
				"## 小红书 style XML 示例",
				"CTA",
				"阅读原文（非正文）",
				"评论设置（非正文）",
			},
		},
		{
			name:     "wechat",
			filename: wechatContractPath,
			title:    "# Genre Contract: WeChat Official Account / 微信公众号文章 (`platform.wechat`)",
			wants: []string{
				"普通微信聊天消息",
				"## 内容模式",
				"知识 / 方法",
				"资讯 / 热点",
				"品牌 / 行动",
				"公众号不是加长版小红书",
				"摘要按需补充关键背景、判断或阅读收益，不复述标题",
				"摘要、导语和首节必须各有信息增量",
				"无来源时不用“多数、普遍、研究表明”等统计口吻",
				"避免短句过多造成逻辑断裂",
				"完整稿至少给出一个封面或正文视觉方案",
				"不设固定数量",
				"不要求固定收尾动作",
			},
			forbidden: []string{
				"## 飞书稿件格式规则",
				"## 微信 style XML 示例",
				"CTA",
				"阅读原文（非正文）",
				"评论设置（非正文）",
				"话题标签（非正文）",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract := readEmbeddedContent(t, test.filename)
			wants := []string{
				test.title,
				"## 适用与消歧",
				"`callout`",
				"Draft Parse Gate 的 `profile.blocks`",
				"飞书源稿禁止使用 `callout`",
			}
			for _, want := range append(wants, test.wants...) {
				if !strings.Contains(contract, want) {
					t.Errorf("platform contract missing %q", want)
				}
			}
			for _, forbidden := range append([]string{
				"route_platform_style",
				"router.xiaohongshu",
				"router.wechat",
				"```",
			}, test.forbidden...) {
				if strings.Contains(contract, forbidden) {
					t.Errorf("platform contract contains removed or cross-platform term %q", forbidden)
				}
			}
		})
	}

	profileTypes := parserProfileBlockTypes(t)
	for _, blockType := range []string{"title", "p", "h2", "h3", "ol", "ul", "li", "img", "table", "callout", "blockquote", "hr"} {
		if !profileTypes[blockType] {
			t.Errorf("platform contract uses non-profile block type %q", blockType)
		}
	}
}

func TestEmbeddedPlatformRouter(t *testing.T) {
	router := readEmbeddedContent(t, platformRouterPath)
	for _, want := range []string{
		"# Genre Router: Platform / 平台发布稿 (`route_platform`)",
		"按目标平台选择且只读取一个 leaf",
		"仅把平台作为研究对象、信息来源或业务渠道时不触发",
		"小红书、小红书写法、小红书 style、红书、XHS、小红书笔记",
		"微信公众号、公众号文章、微信长文、微信爆文",
		"[`xiaohongshu.md`](xiaohongshu.md)",
		"[`wechat.md`](wechat.md)",
	} {
		if !strings.Contains(router, want) {
			t.Errorf("platform router missing %q", want)
		}
	}
	for _, forbidden := range []string{
		"route_platform_style",
		"presentation_mode",
		"Presentation Decision",
		"`callout`",
		"飞书稿件格式规则",
		"```",
	} {
		if strings.Contains(router, forbidden) {
			t.Errorf("platform router contains contract or style rule %q", forbidden)
		}
	}
}

func TestEmbeddedGenresExcludePrintLayoutRules(t *testing.T) {
	forbiddenTerms := []string{
		"A4",
		"天头",
		"订口",
		"版心",
		"页边距",
		"字体",
		"字号",
		"固定行距",
		"每面 22 行",
		"小标宋",
		"仿宋",
		"双面印刷",
		"双面打印",
		"左侧装订",
		"装订",
		"页码",
	}
	for _, filename := range allGenrePaths() {
		content := strings.ReplaceAll(readEmbeddedContent(t, filename), "发文字号", "")
		for _, term := range forbiddenTerms {
			if strings.Contains(content, term) {
				t.Errorf("%s contains print-layout term %q", filename, term)
			}
		}
	}
}

func TestEmbeddedHighRiskGenreRulesRemainConcrete(t *testing.T) {
	tests := map[string][]string{
		"skills/lark-doc/references/genres/official-redhead.md": {
			"决议", "决定", "命令（令）", "公报", "公告", "通告", "意见", "通知", "通报", "报告", "请示", "批复", "议案", "函", "纪要",
			"报告不得夹带请示", "首次引用其他公文", "×政发〔2026〕8号", "`一、`、`（一）`、`1.`、`（1）`", "飞书只交付内容审校稿",
		},
		"skills/lark-doc/references/genres/technical-doc.md": {
			"incident_diagnostic", "授权", "停止", "还原", "blocked",
		},
		"skills/lark-doc/references/genres/sop-tutorial.md": {
			"high-risk", "hold point", "go / no-go", "rollback", "响应预案", "blocked",
		},
		"skills/lark-doc/references/genres/research-report.md": {
			"同意", "匿名", "敏感", "可推广",
		},
		"skills/lark-doc/references/genres/data-report.md": {
			"分子分母", "口径变化", "相关", "因果",
		},
	}
	for filename, markers := range tests {
		content := readEmbeddedContent(t, filename)
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Errorf("%s missing high-risk rule %q", filename, marker)
			}
		}
	}
}

func TestEmbeddedDistilledGenreRulesRemainConcrete(t *testing.T) {
	tests := map[string][]string{
		"skills/lark-doc/references/genres/execution-plan.md": {
			"关键路径", "阶段入口 / 退出条件", "产能", "预警信号", "重新验收",
		},
		"skills/lark-doc/references/genres/sop-tutorial.md": {
			"负责人失联", "断网断电", "进入、升级、降级和解除条件", "联络序列", "故障注入",
		},
		"skills/lark-doc/references/genres/route-knowledge.md": {
			"Learning plan", "完成标准", "调整条件",
		},
		"skills/lark-doc/references/genres/route-consumer.md": {
			"真实 / 有效", "适合当前读者", "值得当前价格", "权重",
		},
		"skills/lark-doc/references/genres/route-marketing.md": {
			"换成竞品名仍成立", "羞耻", "Execution Plan",
		},
		"skills/lark-doc/references/genres/route-opinion.md": {
			"形式选择 → 产生效果", "权限 / 义务",
		},
		"skills/lark-doc/references/genres/route-creative.md": {
			"演员、场地、道具", "固定“前三秒 / 每分钟一反转”",
		},
		"skills/lark-doc/references/genres/formal-doc.md": {
			"听一遍能懂", "朗读校验",
		},
		"skills/lark-doc/references/genres/retrospective.md": {
			"当时判断 → 反证 / 后果 → 新认识 → 下一次可观察行为",
		},
		"skills/lark-doc/references/genres/official-redhead.md": {
			"一个行文方向和授权状态", "待批准后另行制发",
		},
	}
	for filename, markers := range tests {
		content := readEmbeddedContent(t, filename)
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Errorf("%s missing distilled genre rule %q", filename, marker)
			}
		}
	}
}

func TestEmbeddedHighRiskRouteBoundariesRemainConcrete(t *testing.T) {
	tests := map[string][]string{
		"skills/lark-doc/references/genres/route-workplace.md": {
			"表达已核定组织立场",
			"`正式 / 方案 / 计划 / 总结 / 简报 / 讲话稿`单独不触发",
			"明确要求公文 / 红头 / 套红 / 正式发文",
			"法定文种与机关行文关系、文号、主送等制发要素共同出现",
			"`通知 / 报告 / 公告 / 纪要 / 正式 / 官方`单独不触发",
		},
		"skills/lark-doc/references/genres/route-report.md": {
			"正文要求具名决策者选择 / 批准，或形成授权、资源拨付、执行承诺入口时排除",
		},
		"skills/lark-doc/references/genres/business-analysis.md": {
			"正文不要求具名决策者作出选择 / 批准，也不形成授权、资源拨付或执行承诺入口",
		},
		"skills/lark-doc/references/genres/formal-doc.md": {
			"`checkbox`仅用于明确要求的内部执行实例，批准 / 归档版须固定为文字快照",
		},
	}
	for filename, markers := range tests {
		content := readEmbeddedContent(t, filename)
		for _, marker := range markers {
			if !strings.Contains(content, marker) {
				t.Errorf("%s missing route/boundary rule %q", filename, marker)
			}
		}
	}
}

func TestEmbeddedAuthoringWorkflow(t *testing.T) {
	authoring := readEmbeddedContent(t, "skills/lark-doc/references/lark-doc-authoring.md")
	for _, want := range []string{
		"## Philosophy",
		"读者是谁、为什么要读、带着什么任务来",
		"依据关系选择列表、步骤或表格",
		"在文字难以说清流程、交互或层级时用图",
		"`Authoring Brief`",
		"作为进入 Draft 的前置输入",
		"**目标**",
		"**内容**",
		"**边界**",
		"`Presentation Decision`",
		"`presentation_mode` 继承 content contract",
		"`formal`（表达非常正式）",
		"`normal`（正常）",
		"`rich`（表达非常丰富）",
		"mode 不决定 XML / Markdown",
		"不设数量配额",
		"未声明限制的一方不得反推 allow-list",
		"## Route Template Index",
		"### Content routes",
		"| 文件名 | 主要读者任务 |",
		"[`route-workplace.md`](genres/route-workplace.md)",
		"[`route-report.md`](genres/route-report.md)",
		"[`route-platform.md`](genres/route-platform.md)",
		"按目标平台形成可发布成稿",
		"多平台交付共享 contract 与事实基础，但分别执行后续流程",
		"Draft Parse Gate",
		"Parse Gate 只提供语法、基础统计和实际 block 清单，不代表前端视觉验收",
		"Publish Gate = ready",
	} {
		if !strings.Contains(authoring, want) {
			t.Errorf("authoring workflow missing %q", want)
		}
	}
	for _, removed := range []string{
		"比如 / 例如 / 像 / 参考",
		"Gap Projection Map",
		"Source Use Map",
		"Genre Rule Set",
		"Serialization Decision",
		"router_id / contract_id / style_profile_id",
		"Style Profile",
		"### Content contract routes",
		"### Extension routes",
		"| ID |",
		"Route template",
		"Adapter router",
		"_router-",
		"route_platform_style",
		"extension route",
		"可选平台指南",
		"平台指南（如有）",
		"router.xiaohongshu",
		"router.wechat",
		"platform.xiaohongshu",
		"platform.wechat",
		"genres/xiaohongshu.md",
		"genres/wechat.md",
		"style.government",
		"style.institutional",
		"style.knowledge_dev",
		"style.social_content",
		"## Platform Writing Route",
		"对 Draft Parse Gate 无法检测的内容直接检查 release candidate",
		"`align`、行内颜色 / 装饰",
	} {
		if strings.Contains(authoring, removed) {
			t.Errorf("authoring still contains redundant or unverifiable instruction %q", removed)
		}
	}
	philosophyIndex := strings.Index(authoring, "## Philosophy")
	stepPlanIndex := strings.Index(authoring, "## Step Plan")
	if philosophyIndex < 0 || stepPlanIndex < 0 || philosophyIndex > stepPlanIndex {
		t.Error("Philosophy must appear before Step Plan")
	}
	briefIndex := strings.Index(authoring, "形成内部 `Authoring Brief`")
	draftIndex := strings.Index(authoring, "### Draft")
	if briefIndex < 0 || draftIndex < 0 || briefIndex > draftIndex {
		t.Error("Authoring Brief must be completed before entering Draft")
	}
	routeIndex := strings.Index(authoring, "## Route Template Index")
	contentRoutesIndex := strings.Index(authoring, "### Content routes")
	qualityIndex := strings.Index(authoring, "## 质量检测表")
	if routeIndex < stepPlanIndex || contentRoutesIndex < routeIndex || qualityIndex < contentRoutesIndex {
		t.Error("content routes must live inside Route Template Index before the quality table")
	}
	if strings.Contains(authoring, "尚未迁移") {
		t.Error("authoring workflow still contains the legacy-contract migration fallback")
	}
	rootSkill := readEmbeddedContent(t, "skills/lark-doc/SKILL.md")
	if !strings.Contains(rootSkill, "`Presentation Decision` 只确定 `presentation_mode`，不预先选择 block") {
		t.Error("root lark-doc skill must defer block selection until after the presentation decision")
	}
	if strings.Contains(rootSkill, "extension route") {
		t.Error("root lark-doc skill still references removed extension routes")
	}
}

func parseContractRuleTable(t *testing.T, content string) (map[string]string, []string) {
	t.Helper()
	lines := splitMarkdownLines(content)
	if len(lines) < 11 || !strings.HasPrefix(lines[0], "# Genre Contract:") || lines[1] != "" || lines[2] != "## 体裁规则表（硬约束）" || lines[3] != "" {
		t.Fatal("contract must start with title and the unique hard-rule table")
	}
	if strings.Count(content, "## 体裁规则表（硬约束）") != 1 {
		t.Fatal("contract must contain exactly one hard-rule table heading")
	}
	if lines[4] != "| 规则项 | 规则 |" || lines[5] != "|-|-|" {
		t.Fatal("contract hard-rule table must use the canonical two-column header")
	}
	cells := map[string]string{}
	var order []string
	for index := 6; index < len(lines) && lines[index] != ""; index++ {
		columns, ok := markdownTableRow(lines[index])
		if !ok || len(columns) != 2 || columns[0] == "" || columns[1] == "" {
			t.Fatalf("hard-rule row %d must contain a non-empty name and rule, got %q", index-5, lines[index])
		}
		if _, exists := cells[columns[0]]; exists {
			t.Fatalf("hard-rule table contains duplicate row %q", columns[0])
		}
		cells[columns[0]] = columns[1]
		order = append(order, columns[0])
	}
	tableEnd := 6 + len(order)
	if tableEnd >= len(lines) || lines[tableEnd] != "" {
		t.Fatal("hard-rule table must be followed by a blank line")
	}
	return cells, order
}

func splitMarkdownLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	return strings.Split(strings.TrimSuffix(content, "\n"), "\n")
}

func markdownTableRow(line string) ([]string, bool) {
	if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
		return nil, false
	}
	raw := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	parts := strings.Split(raw, "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts, true
}

func assertContractBlockNamesComeFromProfile(t *testing.T, ruleCells map[string]string, rowNames []string) {
	t.Helper()
	profileTypes := parserProfileBlockTypes(t)
	seenPolicy := map[string]string{}
	closedAllowList := slicesContain(rowNames, "允许 block") && slicesContain(rowNames, "少用 block") && slicesContain(rowNames, "禁止 block")
	for _, rowName := range rowNames {
		cell := ruleCells[rowName]
		complementClosed := strings.Contains(cell, "未列入允许") && strings.Contains(cell, "少用") && strings.Contains(cell, "禁止")
		if rowName == "禁止 block" && closedAllowList && !complementClosed {
			t.Error("forbidden block policy must close the allow-list with automatic complement semantics")
		}

		matches := inlineCodePattern.FindAllStringSubmatch(cell, -1)
		if len(matches) == 0 && (rowName != "禁止 block" || !complementClosed) {
			t.Errorf("block policy row %q has no exact profile block type", rowName)
		}
		bareText := inlineCodePattern.ReplaceAllString(cell, "")
		for _, generic := range []string{"图表块", "图片块", "代码块", "表格块", "列表块"} {
			if strings.Contains(bareText, generic) {
				t.Errorf("block policy row %q uses generic type label %q instead of an exact parser type", rowName, generic)
			}
		}
		for _, bareName := range bareASCIINamePattern.FindAllString(bareText, -1) {
			if profileTypes[bareName] {
				t.Errorf("block policy row %q uses parser block type %q without backticks", rowName, bareName)
			}
		}
		for _, match := range matches {
			blockName := match[1]
			if !blockTypePattern.MatchString(blockName) {
				t.Errorf("block policy row %q contains non-type code token %q", rowName, blockName)
				continue
			}
			if previous, exists := seenPolicy[blockName]; exists {
				t.Errorf("block %q appears in both %s and %s", blockName, previous, rowName)
			}
			seenPolicy[blockName] = rowName
			if !profileTypes[blockName] {
				t.Errorf("block name %q is not a profile.blocks[*].type from the parser schema", blockName)
			}
		}
	}
}

func slicesContain(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func assertContractBlockLifecycle(t *testing.T, ruleCells map[string]string) {
	t.Helper()
	allowed := ruleCells["允许 block"]
	if !strings.Contains(allowed, "`title`（完整文稿最多 1 个）") {
		t.Error("strict official-document block policy must allow exactly one title block")
	}
	tablePolicy := allowed + "；" + ruleCells["少用 block"]
	if !strings.Contains(tablePolicy, "`table`") {
		return
	}
	for _, tableType := range []string{"`thead`", "`tbody`", "`tfoot`", "`tr`"} {
		if !strings.Contains(tablePolicy, tableType) {
			t.Errorf("table policy missing profile block type %s", tableType)
		}
	}
}

func parserProfileBlockTypes(t *testing.T) map[string]bool {
	t.Helper()
	profileSource, err := vfs.ReadFile("shortcuts/doc/internal/docxparse/profile.go")
	if err != nil {
		t.Fatalf("read parser profile implementation: %v", err)
	}
	profileFile, err := parser.ParseFile(token.NewFileSet(), "shortcuts/doc/internal/docxparse/profile.go", profileSource, 0)
	if err != nil {
		t.Fatalf("parse parser profile implementation: %v", err)
	}
	if !profileUsesBlockAndRootDualLayouts(profileFile) {
		t.Fatal("profile block selection must include block layouts and root-level dual layouts")
	}

	schemaSource, err := vfs.ReadFile("shortcuts/doc/internal/docxparse/schema.go")
	if err != nil {
		t.Fatalf("read parser schema: %v", err)
	}
	schemaFile, err := parser.ParseFile(token.NewFileSet(), "shortcuts/doc/internal/docxparse/schema.go", schemaSource, 0)
	if err != nil {
		t.Fatalf("parse parser schema: %v", err)
	}
	stringsByName, slicesByName := parserStringBindings(schemaFile)
	blockLayout, blockOK := stringsByName["layoutBlock"]
	dualLayout, dualOK := stringsByName["layoutDual"]
	if !blockOK || !dualOK || blockLayout == dualLayout {
		t.Fatal("parser schema must define distinct block and dual layouts")
	}

	result := map[string]bool{}
	foundLayouts := map[string]bool{}
	ast.Inspect(schemaFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		function, ok := call.Fun.(*ast.Ident)
		if !ok || function.Name != "registerTags" {
			return true
		}
		layout, ok := evalParserString(call.Args[0], stringsByName)
		if !ok || layout != blockLayout && layout != dualLayout {
			return true
		}
		foundLayouts[layout] = true
		for _, argument := range call.Args[1:] {
			if name, ok := evalParserString(argument, stringsByName); ok {
				result[name] = true
				continue
			}
			names, ok := evalParserStringSlice(argument, stringsByName, slicesByName)
			if !ok {
				t.Fatalf("cannot statically resolve parser profile tag argument %T", argument)
			}
			for _, name := range names {
				result[name] = true
			}
		}
		return true
	})
	if !foundLayouts[blockLayout] || !foundLayouts[dualLayout] || len(result) == 0 {
		t.Fatal("parser schema did not expose both block and dual profile types")
	}
	return result
}

func parserStringBindings(file *ast.File) (map[string]string, map[string][]string) {
	expressions := map[string]ast.Expr{}
	for _, declaration := range file.Decls {
		generic, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, specification := range generic.Specs {
			valueSpec, ok := specification.(*ast.ValueSpec)
			if !ok || len(valueSpec.Values) == 0 {
				continue
			}
			for index, name := range valueSpec.Names {
				valueIndex := index
				if len(valueSpec.Values) == 1 {
					valueIndex = 0
				}
				if valueIndex < len(valueSpec.Values) {
					expressions[name.Name] = valueSpec.Values[valueIndex]
				}
			}
		}
	}

	stringsByName := map[string]string{}
	slicesByName := map[string][]string{}
	for pass := 0; pass <= len(expressions); pass++ {
		changed := false
		for name, expression := range expressions {
			if _, known := stringsByName[name]; known {
				continue
			}
			if _, known := slicesByName[name]; known {
				continue
			}
			if value, ok := evalParserString(expression, stringsByName); ok {
				stringsByName[name] = value
				changed = true
				continue
			}
			if values, ok := evalParserStringSlice(expression, stringsByName, slicesByName); ok {
				slicesByName[name] = values
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return stringsByName, slicesByName
}

func evalParserString(expression ast.Expr, stringsByName map[string]string) (string, bool) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return evalParserString(value.X, stringsByName)
	case *ast.BasicLit:
		if value.Kind != token.STRING {
			return "", false
		}
		decoded, err := strconv.Unquote(value.Value)
		return decoded, err == nil
	case *ast.Ident:
		resolved, ok := stringsByName[value.Name]
		return resolved, ok
	case *ast.BinaryExpr:
		if value.Op != token.ADD {
			return "", false
		}
		left, leftOK := evalParserString(value.X, stringsByName)
		right, rightOK := evalParserString(value.Y, stringsByName)
		return left + right, leftOK && rightOK
	default:
		return "", false
	}
}

func evalParserStringSlice(expression ast.Expr, stringsByName map[string]string, slicesByName map[string][]string) ([]string, bool) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return evalParserStringSlice(value.X, stringsByName, slicesByName)
	case *ast.Ident:
		resolved, ok := slicesByName[value.Name]
		return resolved, ok
	case *ast.CompositeLit:
		result := make([]string, 0, len(value.Elts))
		for _, element := range value.Elts {
			if keyed, ok := element.(*ast.KeyValueExpr); ok {
				element = keyed.Value
			}
			item, ok := evalParserString(element, stringsByName)
			if !ok {
				return nil, false
			}
			result = append(result, item)
		}
		return result, true
	case *ast.CallExpr:
		function, ok := value.Fun.(*ast.Ident)
		if !ok || function.Name != "append" || len(value.Args) == 0 {
			return nil, false
		}
		result, ok := evalParserStringSlice(value.Args[0], stringsByName, slicesByName)
		if !ok {
			return nil, false
		}
		result = append([]string{}, result...)
		for _, argument := range value.Args[1:] {
			if item, ok := evalParserString(argument, stringsByName); ok {
				result = append(result, item)
				continue
			}
			items, ok := evalParserStringSlice(argument, stringsByName, slicesByName)
			if !ok {
				return nil, false
			}
			result = append(result, items...)
		}
		return result, true
	default:
		return nil, false
	}
}

func profileUsesBlockAndRootDualLayouts(file *ast.File) bool {
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "buildProfile" {
			continue
		}
		matched := false
		ast.Inspect(function.Body, func(node ast.Node) bool {
			assignment, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for index, left := range assignment.Lhs {
				identifier, ok := left.(*ast.Ident)
				if ok && identifier.Name == "isBlock" && index < len(assignment.Rhs) && isProfileBlockPredicate(assignment.Rhs[index]) {
					matched = true
					return false
				}
			}
			return true
		})
		if matched {
			return true
		}
	}
	return false
}

func isProfileBlockPredicate(expression ast.Expr) bool {
	terms := flattenBinaryExpression(expression, token.LOR)
	hasBlock := false
	hasRootDual := false
	for _, term := range terms {
		if isExpressionEquality(term, "layout", "layoutBlock") {
			hasBlock = true
		}
		andTerms := flattenBinaryExpression(term, token.LAND)
		hasDual := false
		hasRoot := false
		for _, condition := range andTerms {
			hasDual = hasDual || isExpressionEquality(condition, "layout", "layoutDual")
			hasRoot = hasRoot || isExpressionEquality(condition, "node.parent", "nil")
		}
		hasRootDual = hasRootDual || hasDual && hasRoot
	}
	return hasBlock && hasRootDual
}

func flattenBinaryExpression(expression ast.Expr, operator token.Token) []ast.Expr {
	if parenthesized, ok := expression.(*ast.ParenExpr); ok {
		return flattenBinaryExpression(parenthesized.X, operator)
	}
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || binary.Op != operator {
		return []ast.Expr{expression}
	}
	result := flattenBinaryExpression(binary.X, operator)
	return append(result, flattenBinaryExpression(binary.Y, operator)...)
}

func isExpressionEquality(expression ast.Expr, left, right string) bool {
	if parenthesized, ok := expression.(*ast.ParenExpr); ok {
		return isExpressionEquality(parenthesized.X, left, right)
	}
	binary, ok := expression.(*ast.BinaryExpr)
	if !ok || binary.Op != token.EQL {
		return false
	}
	actualLeft, leftOK := expressionName(binary.X)
	actualRight, rightOK := expressionName(binary.Y)
	return leftOK && rightOK && (actualLeft == left && actualRight == right || actualLeft == right && actualRight == left)
}

func expressionName(expression ast.Expr) (string, bool) {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return expressionName(value.X)
	case *ast.Ident:
		return value.Name, true
	case *ast.SelectorExpr:
		prefix, ok := expressionName(value.X)
		if !ok {
			return "", false
		}
		return prefix + "." + value.Sel.Name, true
	default:
		return "", false
	}
}

func allContractPaths() []string {
	result := hardRuleTableContractPaths()
	result = append(result, platformContractPaths...)
	return result
}

func hardRuleTableContractPaths() []string {
	result := append([]string{}, workplaceContractPaths...)
	result = append(result, reportContractPaths...)
	result = append(result, standaloneContractPaths...)
	return result
}

func allGenrePaths() []string {
	result := allContractPaths()
	for filename := range routerLeaves {
		result = append(result, filename)
	}
	sort.Strings(result)
	return result
}

func setDifference(left, right map[string]bool) []string {
	var diff []string
	for item := range left {
		if !right[item] {
			diff = append(diff, item)
		}
	}
	sort.Strings(diff)
	return diff
}

func containsAnySubstring(content string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(content, candidate) {
			return true
		}
	}
	return false
}

func readEmbeddedContent(t *testing.T, filename string) string {
	t.Helper()
	data, err := fs.ReadFile(embeddedContentFS, filename)
	if err != nil {
		t.Fatalf("read embedded content %q: %v", filename, err)
	}
	return string(data)
}
