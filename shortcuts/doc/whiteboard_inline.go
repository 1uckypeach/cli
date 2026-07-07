// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package doc

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	whiteboardTag = "whiteboard"
)

var (
	whiteboardStartTagPattern = regexp.MustCompile(`(?is)<whiteboard\b[^>]*>`)
	whiteboardElementPattern  = regexp.MustCompile(`(?is)<whiteboard\b[^>]*>(.*?)</whiteboard>`)
	whiteboardElementReplacer = regexp.MustCompile(`(?is)<whiteboard\b[^>]*>.*?</whiteboard>`)
)

type whiteboardAttr struct {
	Name  string
	Value string
}

type whiteboardStartTag struct {
	Attrs       []whiteboardAttr
	SelfClosing bool
}

func prepareWhiteboardInlineContent(runtime *common.RuntimeContext, format string, content string) (string, error) {
	if !strings.Contains(content, "<"+whiteboardTag) {
		return content, nil
	}
	if strings.TrimSpace(format) == "markdown" {
		// whiteboard tags are only used in XML format
		return content, nil
	}

	var rewriteErr error
	out := whiteboardElementReplacer.ReplaceAllStringFunc(content, func(raw string) string {
		if rewriteErr != nil {
			return raw
		}
		// Extract the opening tag part
		openTagMatch := whiteboardStartTagPattern.FindString(raw)
		if openTagMatch == "" {
			return raw
		}
		tag, err := parseWhiteboardStartTag(openTagMatch)
		if err != nil {
			rewriteErr = common.ValidationErrorf("invalid whiteboard tag: %v", err).WithParam("whiteboard")
			return raw
		}

		pathValue, hasPath := tag.attr("path")
		if !hasPath {
			// no path attribute, leave as-is
			return raw
		}

		data, err := readWhiteboardPath(runtime, pathValue, "whiteboard path")
		if err != nil {
			rewriteErr = err
			return raw
		}

		// Infer type from extension if not present
		var docType string
		if docType, hasType := tag.attr("type"); hasType {
			docType = strings.TrimSpace(docType)
			if !isValidWhiteboardType(docType) {
				rewriteErr = common.ValidationErrorf("invalid whiteboard type %q; valid types: raw | plantuml | mermaid | svg", docType).WithParam("type")
				return raw
			}
		} else {
			cleanPath := filepath.Clean(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(pathValue), "@")))
			ext := strings.ToLower(filepath.Ext(cleanPath))
			switch ext {
			case ".puml", ".plantuml":
				docType = "plantuml"
			case ".mmd", ".mermaid":
				docType = "mermaid"
			case ".svg":
				docType = "svg"
			default:
				docType = "raw"
			}
		}

		tag.removeAttrs("path")
		if docType != "" {
			if !tag.hasAttr("type") {
				tag.Attrs = append(tag.Attrs, whiteboardAttr{Name: "type", Value: docType})
			}
		} else {
			tag.removeAttrs("type")
		}

		var result strings.Builder
		result.WriteString(tag.render(false))
		result.WriteString(data)
		result.WriteString("</whiteboard>")
		return result.String()
	})

	if rewriteErr != nil {
		return "", rewriteErr
	}
	return out, nil
}

// validateWhiteboardWriteElementBodies ensures that whiteboard tags with path attribute
// don't contain inner content (like html5-block). This prevents ambiguity: you must either
// have the path attribute resolved by CLI OR have content inline, not both.
func validateWhiteboardWriteElementBodies(format string, content string) error {
	validateSegment := func(segment string) error {
		matches := whiteboardElementPattern.FindAllStringSubmatchIndex(segment, -1)
		for _, match := range matches {
			if len(match) < 4 || match[2] < 0 || match[3] < 0 {
				continue
			}
			inner := strings.TrimSpace(segment[match[2]:match[3]])
			if inner != "" {
				// inner content is non-empty — check if there's a path attribute in the opening tag
				raw := segment[match[0]:match[1]]
				tag, err := parseWhiteboardStartTag(raw)
				if err != nil {
					continue // already validated during rewrite; ignore here
				}
				if _, hasPath := tag.attr("path"); hasPath {
					return common.ValidationErrorf("whiteboard with path=\"@...\" cannot contain inner content; remove the content between <whiteboard> and </whiteboard>").WithParam("whiteboard")
				}
			}
		}
		return nil
	}

	if strings.TrimSpace(format) != "markdown" {
		return validateSegment(content)
	}

	var validateErr error
	_ = applyOutsideCodeFences(content, func(segment string) string {
		if validateErr != nil {
			return segment
		}
		if err := validateSegment(segment); err != nil {
			validateErr = err
		}
		return segment
	})
	return validateErr
}

func isValidWhiteboardType(typ string) bool {
	switch typ {
	case "raw", "plantuml", "mermaid", "svg":
		return true
	default:
		return false
	}
}

func readWhiteboardPath(runtime *common.RuntimeContext, pathValue, label string) (string, error) {
	pathRaw := strings.TrimSpace(pathValue)
	if !strings.HasPrefix(pathRaw, "@") {
		return "", common.ValidationErrorf("%s %q must start with @, for example @diagram.puml", label, pathValue).WithParam("path")
	}
	relPath := strings.TrimSpace(strings.TrimPrefix(pathRaw, "@"))
	if relPath == "" {
		return "", common.ValidationErrorf("%s cannot be empty after @", label).WithParam("path")
	}
	clean := filepath.Clean(relPath)
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", common.ValidationErrorf("%s %q must be a relative path within the current working directory", label, pathValue).WithParam("path")
	}
	data, err := cmdutil.ReadInputFile(runtime.FileIO(), clean)
	if err != nil {
		return "", fmt.Errorf("%s %q cannot be read from the current working directory; check that the file exists: %w", label, clean, err)
	}
	return string(data), nil
}

func parseWhiteboardStartTag(raw string) (whiteboardStartTag, error) {
	trimmed := strings.TrimSpace(raw)
	selfClosing := strings.HasSuffix(trimmed, "/>")
	decoder := xml.NewDecoder(strings.NewReader(raw))
	for {
		tok, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return whiteboardStartTag{}, err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != whiteboardTag {
			return whiteboardStartTag{}, fmt.Errorf("expected <%s>, got <%s>", whiteboardTag, start.Name.Local)
		}
		attrs := make([]whiteboardAttr, 0, len(start.Attr))
		for _, attr := range start.Attr {
			attrs = append(attrs, whiteboardAttr{Name: attr.Name.Local, Value: attr.Value})
		}
		return whiteboardStartTag{Attrs: attrs, SelfClosing: selfClosing}, nil
	}
	return whiteboardStartTag{}, fmt.Errorf("missing start element")
}

func (t *whiteboardStartTag) attr(name string) (string, bool) {
	for _, attr := range t.Attrs {
		if attr.Name == name {
			return attr.Value, true
		}
	}
	return "", false
}

func (t *whiteboardStartTag) hasAttr(name string) bool {
	_, ok := t.attr(name)
	return ok
}

func (t *whiteboardStartTag) removeAttrs(names ...string) {
	newAttrs := make([]whiteboardAttr, 0, len(t.Attrs))
	for _, attr := range t.Attrs {
		keep := true
		for _, name := range names {
			if attr.Name == name {
				keep = false
				break
			}
		}
		if keep {
			newAttrs = append(newAttrs, attr)
		}
	}
	t.Attrs = newAttrs
}

func (t whiteboardStartTag) render(selfClosing bool) string {
	var b strings.Builder
	b.WriteByte('<')
	b.WriteString(whiteboardTag)
	for _, attr := range t.Attrs {
		b.WriteByte(' ')
		b.WriteString(attr.Name)
		b.WriteString(`="`)
		b.WriteString(escapeXMLAttr(attr.Value))
		b.WriteByte('"')
	}
	if selfClosing {
		b.WriteString("/>")
	} else {
		b.WriteByte('>')
	}
	if t.SelfClosing && !selfClosing {
		b.WriteString("</")
		b.WriteString(whiteboardTag)
		b.WriteByte('>')
	}
	return b.String()
}

