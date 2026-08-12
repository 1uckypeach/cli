// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

// Package affordance is the lazily-loaded store of usage guidance for
// service-API methods. The source of truth is one markdown file per service in
// the top-level affordance/ tree (see mdparse.go), injected via SetSource so
// domain owners maintain it next to skills/ and shortcuts/. A service is read
// and parsed at most once, on first access, so normal command execution never
// touches it.
package affordance

import (
	"encoding/json"
	"io/fs"
	"strings"
	"sync"

	"github.com/larksuite/cli/internal/apicatalog"
)

var (
	mu        sync.Mutex
	byService = map[string]parsedDomain{}
	mdSource  fs.FS // top-level affordance/*.md tree; nil in the minimal preview build
)

// SetSource installs the markdown guidance tree (the top-level affordance/
// directory) as the source. Called once at startup before any lookup; clears
// the parse cache so re-sourcing (e.g. in tests) takes effect.
func SetSource(fsys fs.FS) {
	mu.Lock()
	defer mu.Unlock()
	mdSource = fsys
	byService = map[string]parsedDomain{}
}

// For returns the raw affordance overlay for one method, loading the owning
// service on first access. ok is false when there is no entry (absent source,
// parse failure, or unknown method all collapse to "no guidance").
func For(catalog apicatalog.Catalog, service, methodID string) (json.RawMessage, bool) {
	mu.Lock()
	defer mu.Unlock()
	parsed, ok := sourceForService(service)
	if !ok {
		return nil, false
	}
	a, ok := resolveParsedDomain(parsed, commandFormResolver(catalog, service)).methods[methodID]
	if !ok {
		return nil, false
	}
	raw, err := json.Marshal(a)
	return raw, err == nil && len(raw) > 0
}

// DomainSkill returns the service-level canonical skill declared by
// `> skill:`. That declaration is independent of method command mappings.
func DomainSkill(service string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	parsed, ok := sourceForService(service)
	if !ok {
		return "", false
	}
	skill := parsed.skill
	return skill, skill != ""
}

// DomainSkills returns the skill references configured for service-level help.
// The canonical `> skill:` entry is first when present, followed by entries in
// the domain's `## Skills` section. The returned slice is a copy so callers
// cannot mutate the lazy parse cache.
func DomainSkills(service string) ([]string, bool) {
	mu.Lock()
	defer mu.Unlock()
	parsed, ok := sourceForService(service)
	if !ok {
		return nil, false
	}
	skills := parsed.domainSkills
	if len(skills) == 0 {
		return nil, false
	}
	return append([]string(nil), skills...), true
}

// sourceForService caches only stable markdown-derived source keyed by service.
// Catalog-specific command-form mappings are resolved at lookup time, so short-
// lived Catalog instances cannot become process-global cache keys.
func sourceForService(service string) (parsedDomain, bool) {
	if parsed, ok := byService[service]; ok {
		return parsed, true
	}
	if mdSource == nil {
		return parsedDomain{}, false
	}
	src, err := fs.ReadFile(mdSource, service+".md")
	if err != nil {
		return parsedDomain{}, false
	}
	parsed := parseRawDomainMD(src)
	byService[service] = parsed
	return parsed, true
}

// commandFormResolver maps a method's command-form heading ("user_mailbox.messages
// list") to its method id ("user_mailbox.message.list") via the injected catalog's
// authoritative resource↔id table. Resource names are irregularly pluralised
// (message/messages, user_mailbox/user_mailboxes), so this cannot be guessed; the
// space→dot fallback covers domains where the two already coincide.
func commandFormResolver(catalog apicatalog.Catalog, service string) func(string) string {
	byForm := map[string]string{}
	if svc, ok := catalog.Service(service); ok {
		for _, ref := range apicatalog.ServiceMethods(svc, nil) {
			byForm[strings.Join(ref.CommandPath()[1:], " ")] = ref.Method.ID
		}
	}
	return func(h string) string {
		if id, ok := byForm[strings.TrimSpace(h)]; ok {
			return id
		}
		return headingToKey(h) // one home for the shortcut/method key convention
	}
}
