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
	byCatalog = map[catalogCacheKey]serviceAffordance{}
	tried     = map[catalogCacheKey]bool{}
	mdSource  fs.FS // top-level affordance/*.md tree; nil in the minimal preview build
)

type catalogCacheKey struct {
	catalog apicatalog.Identity
	service string
}

type serviceAffordance struct {
	skill        string
	domainSkills []string
	methods      map[string]json.RawMessage
}

// SetSource installs the markdown guidance tree (the top-level affordance/
// directory) as the source. Called once at startup before any lookup; clears
// the parse cache so re-sourcing (e.g. in tests) takes effect.
func SetSource(fsys fs.FS) {
	mu.Lock()
	defer mu.Unlock()
	mdSource = fsys
	byCatalog = map[catalogCacheKey]serviceAffordance{}
	tried = map[catalogCacheKey]bool{}
}

// For returns the raw affordance overlay for one method, loading the owning
// service on first access. ok is false when there is no entry (absent source,
// parse failure, or unknown method all collapse to "no guidance").
func For(catalog apicatalog.Catalog, service, methodID string) (json.RawMessage, bool) {
	mu.Lock()
	defer mu.Unlock()
	key := cacheKey(catalog, service)
	if !tried[key] {
		tried[key] = true
		byCatalog[key] = loadService(catalog, service)
	}
	raw, ok := byCatalog[key].methods[methodID]
	return raw, ok && len(raw) > 0
}

// DomainSkill returns the service-level canonical skill declared by
// `> skill:`. That declaration is independent of method command mappings.
func DomainSkill(service string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	key := cacheKey(apicatalog.Catalog{}, service)
	if !tried[key] {
		tried[key] = true
		byCatalog[key] = loadService(apicatalog.Catalog{}, service)
	}
	skill := byCatalog[key].skill
	return skill, skill != ""
}

// DomainSkills returns the skill references configured for service-level help.
// The canonical `> skill:` entry is first when present, followed by entries in
// the domain's `## Skills` section. The returned slice is a copy so callers
// cannot mutate the lazy parse cache.
func DomainSkills(service string) ([]string, bool) {
	mu.Lock()
	defer mu.Unlock()
	key := cacheKey(apicatalog.Catalog{}, service)
	if !tried[key] {
		tried[key] = true
		byCatalog[key] = loadService(apicatalog.Catalog{}, service)
	}
	skills := byCatalog[key].domainSkills
	if len(skills) == 0 {
		return nil, false
	}
	return append([]string(nil), skills...), true
}

// cacheKey identifies the immutable Catalog instance that owns command-form
// mapping. Catalog.Identity is fixed-size and O(1), so schema assembly does not
// rebuild and hash the service's complete method mapping for every lookup.
func cacheKey(catalog apicatalog.Catalog, service string) catalogCacheKey {
	return catalogCacheKey{catalog: catalog.Identity(), service: service}
}

// loadService parses a service's markdown guidance into its domain metadata
// and per-method overlays, marshalling each method to JSON so downstream
// callers keep the same wire shape.
func loadService(catalog apicatalog.Catalog, service string) serviceAffordance {
	if mdSource == nil {
		return serviceAffordance{}
	}
	src, err := fs.ReadFile(mdSource, service+".md")
	if err != nil {
		return serviceAffordance{}
	}
	parsed := parseDomainMD(src, commandFormResolver(catalog, service))
	methods := map[string]json.RawMessage{}
	for id, a := range parsed.methods {
		if b, err := json.Marshal(a); err == nil {
			methods[id] = b
		}
	}
	return serviceAffordance{
		skill:        parsed.skill,
		domainSkills: parsed.domainSkills,
		methods:      methods,
	}
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
