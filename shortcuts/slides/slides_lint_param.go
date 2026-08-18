// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import "github.com/larksuite/cli/shortcuts/common"

// The server-side XML lint switch, carried by every shortcut that writes a
// whole <slide> document: +create, +add-slide and +update-slide.
//
// The backend lints the page it is handed and refuses the write when the page
// fails. The CLI asks for that on every call, so a caller who never touches the
// flag gets the checked path; --no-lint is the escape hatch for the case the
// lint is wrong about a page that must still ship.
//
// Sent explicitly in both directions rather than omitted when on: the parameter
// is newer than internal/registry/meta_data.json, so the server-side default is
// not something this CLI can read anywhere, and a request that states the value
// means the same thing before and after that default ever changes.
const noLintFlagName = "no-lint"

// lintXMLParamKey is the wire name the switch binds to. It sits alone in a
// constant because it is the one part of this change that cannot be checked
// against the public API registry: xml_presentation.slide.create/replace do not
// list a lint parameter, so this name follows the convention of its neighbours
// rather than a published definition.
//
// snake_case because every parameter the registry does define is snake_case —
// 110 of them across all services, none camelCase. The closest match is
// remove_attr_id on xml_presentations.get: same service, same shape (a boolean
// switch), spelled the same way.
//
// Worth knowing when this is next touched: a wrong name here fails silently.
// The server ignores query parameters it does not recognise, so --no-lint would
// go through, return success, and lint the page anyway. Tests and --dry-run
// cannot catch that — they only prove what the CLI sent, not what was read.
const lintXMLParamKey = "lint_xml"

// noLintFlag is the shared flag definition, so the three commands cannot drift
// apart on the name or the wording.
func noLintFlag() common.Flag {
	return common.Flag{
		Name: noLintFlagName,
		Type: "bool",
		Desc: "submit the XML without the server-side lint; by default the server lints every page and rejects the write when it fails",
	}
}

// withLintXML stamps the switch onto a query map and returns it, so the query
// builders stay one-liners and dry-run and execute cannot disagree about the
// value. A nil map is allocated rather than rejected: +create has calls that
// carry no other query parameter.
func withLintXML(query map[string]interface{}, runtime *common.RuntimeContext) map[string]interface{} {
	if query == nil {
		query = map[string]interface{}{}
	}
	query[lintXMLParamKey] = !runtime.Bool(noLintFlagName)
	return query
}
