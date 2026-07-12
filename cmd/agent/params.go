// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// This file is the per-verb business-parameter engine: --param k=v parsing,
// collect-all validation against one operation's declared set (every violation
// reported in one pass, each self-contained enough to fix without a discovery
// round-trip), default backfill, and the meta.next carry rule.

package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/errs"
	iagent "github.com/larksuite/cli/internal/agent"
)

// flatParams expands declarations to value-bearing leaves: scalars keep their
// name, an object contributes one leaf per Field under "obj.field" dotted
// names (leaf attributes rule). The object entry itself is NOT value-bearing
// and is excluded. Order is declaration order (meta.next determinism).
func flatParams(declared []iagent.CardParam) []iagent.CardParam {
	out := make([]iagent.CardParam, 0, len(declared))
	for _, cp := range declared {
		if cp.Type == "object" {
			for _, f := range cp.Fields {
				leaf := f
				leaf.Name = cp.Name + "." + f.Name
				out = append(out, leaf)
			}
			continue
		}
		out = append(out, cp)
	}
	return out
}

// objectDecls indexes the top-level object params by name.
func objectDecls(declared []iagent.CardParam) map[string]iagent.CardParam {
	out := map[string]iagent.CardParam{}
	for _, cp := range declared {
		if cp.Type == "object" {
			out[cp.Name] = cp
		}
	}
	return out
}

// validatedParams is the engine's product: Resolved is what the runtime hands
// to the provider hook (defaults backfilled); Given is only what the caller
// explicitly provided (no defaults) — the meta.next carry rule reads Given so
// backfilled defaults never turn into command-line noise.
type validatedParams struct {
	Resolved map[string]string
	Given    map[string]string
}

// addParamFlag registers the shared --param flag on a leaf (two-line helper,
// same style as addAsFlag).
func addParamFlag(cmd *cobra.Command, params *[]string) {
	cmd.Flags().StringArrayVar(params, "param", nil, "业务参数 key=value，可重复（各命令所需参数见 lark-cli agent card <agent_ref> --operation <verb>）")
}

// validateParams parses --param pairs and validates them against ONE
// operation's declared parameter set, collecting ALL violations into a single
// typed invalid_argument error (exit 2). spec is used for the cross-operation
// reverse lookup on unknown keys ("它声明在: send") and may be nil (agent list
// path). Passing validation backfills declaration defaults into Resolved.
func validateParams(kvs []string, declared []iagent.CardParam, verb string, spec *iagent.AgentSpec, ref string) (validatedParams, error) {
	// decl indexes the value-bearing leaves: scalars by name, object fields by
	// dotted "obj.field" names — the canonical flat form every downstream
	// consumer (Resolved, meta.next, rt.Params()) speaks.
	leaves := flatParams(declared)
	decl := make(map[string]iagent.CardParam, len(leaves))
	for _, p := range leaves {
		decl[p.Name] = p
	}
	objects := objectDecls(declared)

	// seen 记录“这个 key 在 argv 里出现过”（重复检测 + 抑制误报的 missing-
	// required 都看它）；given 只收录通过校验的值（Resolved/meta.next 都看它）。
	// 两张表必须分开：值校验失败的 key 若不进 seen，重复提供会漏报、缺必填会误报
	// （参数明明给了、只是值不对，再报一条“缺少必填”是自相矛盾的指令）。
	// objChannel 记录每个对象走的通道（dotted|json），同一对象混用两通道报错，
	// 不做静默合并。
	seen := map[string]bool{}
	given := map[string]string{}
	objChannel := map[string]string{}
	var viols []errs.InvalidParam

	addViol := func(name, reason string, spec *iagent.CardParam, suggestions ...string) {
		v := errs.InvalidParam{Name: name, Reason: reason, Suggestions: suggestions}
		if spec != nil {
			v.Spec = *spec
		}
		viols = append(viols, v)
	}

	// ── parse + per-key checks（一次收集全部）──
	for _, kv := range kvs {
		k, val, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			addViol(kv, fmt.Sprintf("--param 格式应为 key=value，得到 %q", kv), nil)
			continue
		}
		if seen[k] {
			addViol(k, fmt.Sprintf("参数 %s 重复提供（该参数不可重复）", k), nil)
			continue
		}
		seen[k] = true

		// ── 对象的 JSON 整值通道：key 恰是对象名 ──
		if obj, isObj := objects[k]; isObj {
			if objChannel[k] == "dotted" {
				addViol(k, fmt.Sprintf("参数 %s 以 JSON 与点路径混合提供（同一对象只能选一种通道）", k), nil)
				continue
			}
			objChannel[k] = "json"
			validateObjectJSON(k, val, obj, verb, seen, given, addViol)
			continue
		}

		// ── 点路径通道：key 带 "."，指向对象的某个叶子 ──
		if top, leaf, dotted := strings.Cut(k, "."); dotted {
			obj, isObj := objects[top]
			if !isObj {
				reason, sugg := unknownParamReason(k, verb, leaves, spec)
				addViol(k, reason, nil, sugg...)
				continue
			}
			if objChannel[top] == "json" {
				addViol(k, fmt.Sprintf("参数 %s 以 JSON 与点路径混合提供（同一对象只能选一种通道）", top), nil)
				continue
			}
			objChannel[top] = "dotted"
			cp, known := decl[k]
			if !known {
				addViol(k, fmt.Sprintf("未知参数 %s（%s 可用字段: %s）", k, top, fieldNames(obj)), nil, dottedFieldNames(obj)...)
				continue
			}
			_ = leaf
			if val == "" {
				if cp.Required {
					addViol(k, fmt.Sprintf("必填参数 %s 不能为空值（%s 必填）", k, verb), &cp)
				}
				continue
			}
			if err := iagent.ValidateValue(cp, val); err != nil {
				addViol(k, fmt.Sprintf("参数 %s %s", k, err.Error()), &cp, cp.Enum...)
				continue
			}
			given[k] = canonicalValue(cp, val)
			continue
		}

		cp, known := decl[k]
		if !known {
			reason, sugg := unknownParamReason(k, verb, leaves, spec)
			addViol(k, reason, nil, sugg...)
			continue
		}
		if val == "" {
			// `k=` 空值统一按“未提供”处理（不进 given ⇒ 不遮蔽 Default 回填、
			// 不把未过 Type/Enum/Range 校验的 "" 交给 hook——rt.Params() 契约）。
			// 必填参数额外报专属违规；可选参数省略即得默认值，无需报错。
			if cp.Required {
				addViol(k, fmt.Sprintf("必填参数 %s 不能为空值（%s 必填）", k, verb), &cp)
			}
			continue
		}
		if err := iagent.ValidateValue(cp, val); err != nil {
			addViol(k, fmt.Sprintf("参数 %s %s", k, err.Error()), &cp, cp.Enum...)
			continue
		}
		given[k] = canonicalValue(cp, val)
	}

	// ── missing required（对着平铺声明反查；argv 里出现过的 key 不再重复报——
	// 它要么已通过、要么已带着更精确的违规）──
	for _, cp := range leaves {
		if !cp.Required || seen[cp.Name] {
			continue
		}
		c := cp
		addViol(cp.Name, fmt.Sprintf("缺少必填参数 %s（%s 必填）", cp.Name, verb), &c)
	}

	if len(viols) > 0 {
		return validatedParams{}, paramsError(viols, verb, ref)
	}

	// ── default 回填（只作用于完全缺席的键）──
	resolved := make(map[string]string, len(given))
	for k, v := range given {
		resolved[k] = v
	}
	for _, cp := range leaves {
		if cp.Default == "" {
			continue
		}
		if _, ok := resolved[cp.Name]; !ok {
			resolved[cp.Name] = cp.Default
		}
	}
	return validatedParams{Resolved: resolved, Given: given}, nil
}

// validateObjectJSON is the JSON fallback channel: parse the value as a JSON
// object, validate each member against the declared Fields with the SAME leaf
// rules as the dotted channel, and normalize accepted members into flat dotted
// keys — a provider never sees which channel the caller used. Numbers decode
// via json.Number so "100" stays "100" (no float re-rendering).
func validateObjectJSON(name, val string, obj iagent.CardParam, verb string, seen map[string]bool, given map[string]string, addViol func(string, string, *iagent.CardParam, ...string)) {
	if val == "" {
		return // `obj=` 空值 = 未提供（与标量语义一致）
	}
	dec := json.NewDecoder(strings.NewReader(val))
	dec.UseNumber()
	var anyVal any
	if err := dec.Decode(&anyVal); err != nil {
		addViol(name, fmt.Sprintf("参数 %s 的 JSON 无法解析（%v）；也可用点路径逐字段传：--param %s.<field>=<value>", name, err, name), nil)
		return
	}
	raw, isObj := anyVal.(map[string]any)
	if !isObj {
		// 语法合法但不是对象（数组/字符串/数字/布尔/null）——用调用方词汇描述，
		// 不泄漏 Go 反序列化的内部类型文案。
		addViol(name, fmt.Sprintf(`参数 %s 需为 JSON 对象（如 {"k":"v"}），得到 %s；也可用点路径逐字段传：--param %s.<field>=<value>`, name, jsonKindName(anyVal), name), nil)
		return
	}
	fields := map[string]iagent.CardParam{}
	for _, f := range obj.Fields {
		fields[f.Name] = f
	}
	for fk, fv := range raw {
		full := name + "." + fk
		seen[full] = true
		cp, ok := fields[fk]
		if !ok {
			addViol(full, fmt.Sprintf("未知参数 %s（%s 可用字段: %s）", full, name, fieldNames(obj)), nil, obj.FieldNamesList()...)
			continue
		}
		var sval string
		switch tv := fv.(type) {
		case string:
			sval = tv
		case json.Number:
			sval = tv.String()
		case bool:
			sval = strconv.FormatBool(tv)
		case nil:
			continue // null = 未提供
		default:
			addViol(full, fmt.Sprintf("参数 %s 不支持嵌套结构（对象字段只能是标量）", full), &cp)
			continue
		}
		if sval == "" {
			if cp.Required {
				c := cp
				c.Name = fk
				addViol(full, fmt.Sprintf("必填参数 %s 不能为空值（%s 必填）", full, verb), &c)
			}
			continue
		}
		if err := iagent.ValidateValue(cp, sval); err != nil {
			c := cp
			addViol(full, fmt.Sprintf("参数 %s %s", full, err.Error()), &c, cp.Enum...)
			continue
		}
		given[full] = canonicalValue(cp, sval)
	}
}

// fieldNames renders an object's field list for teaching errors.
func fieldNames(obj iagent.CardParam) string {
	return strings.Join(obj.FieldNamesList(), ", ")
}

// unknownParamReason builds the teaching sentence for an undeclared key: if
// another operation of the same spec declares it, name those operations（改动
// 词就能修）；otherwise list this operation's own parameter set（改拼写就能修）.
func unknownParamReason(key, verb string, declared []iagent.CardParam, spec *iagent.AgentSpec) (string, []string) {
	if spec != nil {
		var elsewhere []string
		for _, o := range spec.Ops() {
			if o.Verb == verb || !o.Wired {
				continue
			}
			for _, p := range flatParams(o.Params) {
				if p.Name == key {
					elsewhere = append(elsewhere, o.Verb)
					break
				}
			}
		}
		if len(elsewhere) > 0 {
			sort.Strings(elsewhere)
			// suggestions 保持单一语义（可直接替换的参数名候选）：动词名不是参数，
			// 不进 suggestions——「声明在: X」的教学已在 reason 里。
			return fmt.Sprintf("参数 %s 不适用于 %s（它声明在: %s）", key, verb, strings.Join(elsewhere, ", ")), nil
		}
	}
	known := make([]string, 0, len(declared))
	for _, p := range declared {
		known = append(known, p.Name)
	}
	if len(known) == 0 {
		return fmt.Sprintf("未知参数 %s（%s 不接受任何业务参数）", key, verb), nil
	}
	// suggestions 按编辑距离给「可直接替换」的近似候选（typo 一步可修）；
	// 没有近似命中时退回声明序全集。message 始终列全集（发现面完整）。
	sugg := nearestNames(key, known, 2)
	if len(sugg) == 0 {
		sugg = known
	}
	return fmt.Sprintf("未知参数 %s（%s 可用参数: %s）", key, verb, strings.Join(known, ", ")), sugg
}

// nearestNames returns the candidates within maxDist Levenshtein distance of
// key, nearest first (stable for ties by candidate order).
func nearestNames(key string, candidates []string, maxDist int) []string {
	type scored struct {
		name string
		d    int
	}
	var hits []scored
	for _, c := range candidates {
		if d := levenshtein(key, c); d <= maxDist {
			hits = append(hits, scored{c, d})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].d < hits[j].d })
	out := make([]string, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.name)
	}
	return out
}

// levenshtein is the classic two-row edit distance over runes.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(min(cur[j-1]+1, prev[j]+1), prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// jsonKindName names a decoded JSON value's kind in caller vocabulary.
func jsonKindName(v any) string {
	switch v.(type) {
	case []any:
		return "数组"
	case string:
		return "字符串"
	case json.Number:
		return "数字"
	case bool:
		return "布尔值"
	case nil:
		return "null"
	default:
		return "非对象值"
	}
}

// canonicalValue normalizes an ACCEPTED scalar to its canonical wire form so a
// provider receives one deterministic literal regardless of the input variant
// or channel: boolean TRUE/1/t → true|false, integer +5/04 → 5/4. The JSON
// channel already produces canonical literals for native types; this closes
// the dotted-path (and JSON string-member) variants to the same form. Values
// that reach here have passed ValidateValue, so parse errors are impossible;
// the input is returned unchanged as a defensive fallback.
func canonicalValue(cp iagent.CardParam, val string) string {
	switch cp.Type {
	case "boolean":
		if b, err := strconv.ParseBool(val); err == nil {
			return strconv.FormatBool(b)
		}
	case "integer":
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			return strconv.FormatInt(n, 10)
		}
	case "number":
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return strconv.FormatFloat(f, 'g', -1, 64)
		}
	}
	return val
}

// dottedFieldNames returns an object's field names in their full dotted form
// (directly substitutable --param keys).
func dottedFieldNames(obj iagent.CardParam) []string {
	out := make([]string, 0, len(obj.Fields))
	for _, f := range obj.Fields {
		out = append(out, obj.Name+"."+f.Name)
	}
	return out
}

// paramsError folds collected violations into one typed error: a single
// violation keeps its sentence as the message (continuity with the old
// one-error style); several get a count summary, with every violation carried
// structurally in params[].
func paramsError(viols []errs.InvalidParam, verb, ref string) error {
	msg := viols[0].Reason
	if len(viols) > 1 {
		msg = fmt.Sprintf("%s 参数校验失败：%d 处问题（详见 params）", verb, len(viols))
	}
	e := errs.NewValidationError(errs.SubtypeInvalidArgument, "%s", msg).
		WithParam("param:" + viols[0].Name).
		WithParams(viols...)
	return e.WithHint("%s", opHint(ref, verb))
}

// validateListParams is the `agent list <scheme>` variant of validateParams:
// list is a provider-level operation with no agent_ref yet, so there is no
// spec for cross-operation reverse lookup, and the discovery hint points at
// the provider listing's list_parameters instead of an agent card.
func validateListParams(kvs []string, declared []iagent.CardParam, scheme string) (validatedParams, error) {
	vp, err := validateParams(kvs, declared, "list", nil, "")
	if err != nil {
		var verr *errs.ValidationError
		if errors.As(err, &verr) {
			verr.Hint = fmt.Sprintf("按 params 逐条修正后重发；agent list %s 的可用参数见 lark-cli agent list 输出的 providers[].list_parameters", scheme)
		}
		return validatedParams{}, err
	}
	return vp, nil
}

// opHint is the operation-scoped discovery hint（ref 过白名单才内插命令）.
func opHint(ref, verb string) string {
	if safeNextRef(ref) {
		return fmt.Sprintf("按 params 逐条修正后重发；或运行 lark-cli agent card %s --operation %s 查看参数声明", ref, verb)
	}
	return "按 params 逐条修正后重发；或用 agent card 的 --operation 子查询查看参数声明"
}

// paramArgsFor renders the meta.next carry for target verb V per the
// three-way rule, in declaration order:
//  1. given + value passes the whitelist → carry literally;
//  2. given + value fails the whitelist → required degrades to a placeholder
//     (template), optional is dropped（宁缺毋歧义）;
//  3. absent but required on V → placeholder (template) — the cross-verb hole:
//     without this, "链上不丢必填" only holds when the previous verb happened
//     to share the parameter.
//
// Defaults are NOT carried (the next hop deterministically re-backfills).
func paramArgsFor(spec *iagent.AgentSpec, verb string, given map[string]string) (args string, templated bool) {
	if spec == nil {
		return "", false
	}
	op, ok := spec.Op(verb)
	if !ok {
		return "", false
	}
	var b strings.Builder
	for _, p := range flatParams(op.Params) {
		v, has := given[p.Name]
		switch {
		case p.NoCarry:
			// 每次调用应给新值的参数：给过也不字面上链；必填的降级占位，提醒
			// 调用方填一个新值（而不是复用上一次的）。
			if p.Required {
				fmt.Fprintf(&b, " --param %s=<%s>", p.Name, p.Name)
				templated = true
			}
		case has && v != "" && safeNextID(v):
			fmt.Fprintf(&b, " --param %s=%s", p.Name, v)
		case p.Required:
			fmt.Fprintf(&b, " --param %s=<%s>", p.Name, p.Name)
			templated = true
		}
	}
	return b.String(), templated
}
