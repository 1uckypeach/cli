// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package service

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/larksuite/cli/internal/affordance"
	"github.com/larksuite/cli/internal/apicatalog"
	"github.com/larksuite/cli/internal/cmdmeta"
	"github.com/larksuite/cli/internal/meta"
	"github.com/larksuite/cli/internal/skillref"
	"github.com/spf13/cobra"
)

func TestLegacyAffordanceHelpSignaturesCompileAndCall(t *testing.T) {
	var method func(*cobra.Command, fs.FS) bool = PrepareMethodHelp
	var methodRefs func(*cobra.Command, fs.FS, *skillref.Resolver) bool = PrepareMethodHelpWithReferences
	var methodProjection func(*cobra.Command, fs.FS, *skillref.Resolver, func() bool) bool = PrepareMethodHelpWithProjection
	var shortcut func(*cobra.Command, fs.FS) bool = PrepareShortcutHelp
	var shortcutRefs func(*cobra.Command, fs.FS, *skillref.Resolver) bool = PrepareShortcutHelpWithReferences

	plain := &cobra.Command{Use: "plain"}
	if method(plain, nil) || methodRefs(plain, nil, nil) ||
		methodProjection(plain, nil, nil, nil) || shortcut(plain, nil) ||
		shortcutRefs(plain, nil, nil) {
		t.Fatal("legacy help wrappers accepted an unannotated command")
	}
}

func TestPrepareMethodHelpCatalogUsesInjectedIrregularCommandForm(t *testing.T) {
	affordance.SetSource(fstest.MapFS{
		"drive.md": {Data: []byte("# drive\n\n## files list\nList files through the injected mapping.\n")},
	})
	t.Cleanup(func() { affordance.SetSource(nil) })

	service := meta.ServiceFromMap(map[string]interface{}{
		"name": "drive",
		"resources": map[string]interface{}{
			"files": map[string]interface{}{
				"methods": map[string]interface{}{
					"list": map[string]interface{}{"id": "file.list", "httpMethod": "GET"},
				},
			},
		},
	})
	catalog := apicatalog.New(apicatalog.SourceEmbedded, []meta.Service{service})
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List files",
		Annotations: map[string]string{
			schemaPathAnnotation: "drive.files.list",
		},
	}
	cmdmeta.SetAffordanceRef(cmd, "drive", "file.list")

	if !PrepareMethodHelpCatalog(catalog, cmd, nil) {
		t.Fatal("PrepareMethodHelpCatalog rejected a method command")
	}
	if !strings.Contains(cmd.Long, "List files through the injected mapping.") {
		t.Fatalf("catalog-aware help lost the irregular command-form mapping:\n%s", cmd.Long)
	}
}
