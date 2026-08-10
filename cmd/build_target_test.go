// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/platform"
	"github.com/larksuite/cli/internal/apicatalog"
	"github.com/larksuite/cli/internal/cmdpolicy"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/registry"
	"github.com/larksuite/cli/shortcuts"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type recordingSnapshot struct {
	delegate      *registry.Snapshot
	catalogCalls  int
	fullCalls     int
	catalogNames  [][]string
	beforeCatalog func()
}

func (s *recordingSnapshot) ServiceNames() []string {
	return s.delegate.ServiceNames()
}

func (s *recordingSnapshot) Catalog(names ...string) (apicatalog.Catalog, error) {
	if s.beforeCatalog != nil {
		s.beforeCatalog()
	}
	s.catalogCalls++
	s.catalogNames = append(s.catalogNames, append([]string(nil), names...))
	return s.delegate.Catalog(names...)
}

func (s *recordingSnapshot) FullCatalog() (apicatalog.Catalog, error) {
	if s.beforeCatalog != nil {
		s.beforeCatalog()
	}
	s.fullCalls++
	return s.delegate.FullCatalog()
}

func newRecordingSnapshot(t testing.TB) *recordingSnapshot {
	t.Helper()
	snapshot, err := registry.OpenSnapshot()
	if err != nil {
		t.Fatalf("OpenSnapshot: %v", err)
	}
	return &recordingSnapshot{delegate: snapshot}
}

func withRecordingSnapshot(snapshot catalogSnapshot, opens *int) BuildOption {
	return func(cfg *buildConfig) {
		cfg.snapshotOpener = func() (catalogSnapshot, error) {
			*opens++
			return snapshot, nil
		}
	}
}

func quietBuildOptions(snapshot catalogSnapshot, opens *int) []BuildOption {
	return []BuildOption{
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutPlugins(),
		WithoutStrictMode(),
		withRecordingSnapshot(snapshot, opens),
	}
}

func TestBuildForArgsAssemblyLoading(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantOpens    int
		wantCatalogs [][]string
		wantFull     int
	}{
		{name: "version", args: []string{"--version"}, wantOpens: 0},
		{name: "target api", args: []string{"drive", "files", "list"}, wantOpens: 1, wantCatalogs: [][]string{{"drive"}}},
		{name: "target schema", args: []string{"schema", "drive.file.comments.list"}, wantOpens: 1, wantCatalogs: [][]string{{"drive"}}},
		{name: "target schema service", args: []string{"schema", "drive"}, wantOpens: 1, wantCatalogs: [][]string{{"drive"}}},
		// The service index is answerable from the manifest alone, so the
		// manifest is opened but no shard is ever parsed.
		{name: "bare schema", args: []string{"schema"}, wantOpens: 1},
		{name: "shortcut only", args: []string{"docs", "+fetch"}, wantOpens: 1, wantCatalogs: [][]string{nil}},
		{name: "root help", args: []string{"--help"}, wantOpens: 1, wantFull: 1},
		{name: "completion", args: []string{"completion", "zsh"}, wantOpens: 1, wantFull: 1},
		{name: "hand authored", args: []string{"api", "GET", "/open-apis/test"}, wantOpens: 1, wantFull: 1},
		{name: "ambiguous", args: []string{"--unknown", "drive"}, wantOpens: 1, wantFull: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := newRecordingSnapshot(t)
			opens := 0
			_, err := BuildForArgs(
				context.Background(),
				cmdutil.InvocationContext{},
				tt.args,
				quietBuildOptions(snapshot, &opens)...,
			)
			if err != nil {
				t.Fatalf("BuildForArgs: %v", err)
			}
			if opens != tt.wantOpens {
				t.Fatalf("Snapshot opens = %d, want %d", opens, tt.wantOpens)
			}
			if !reflect.DeepEqual(snapshot.catalogNames, tt.wantCatalogs) {
				t.Errorf("Catalog calls = %#v, want %#v", snapshot.catalogNames, tt.wantCatalogs)
			}
			if snapshot.fullCalls != tt.wantFull {
				t.Errorf("FullCatalog calls = %d, want %d", snapshot.fullCalls, tt.wantFull)
			}
			if snapshot.catalogCalls+snapshot.fullCalls > 1 {
				t.Errorf("Catalog selected %d times, want at most once", snapshot.catalogCalls+snapshot.fullCalls)
			}
		})
	}
}

// The name-only index assembly is a saving, not a different answer. The
// flag-first form reaches the same listing through the full catalog, which
// makes it the control: same bytes out, one of them without touching a shard.
func TestBareSchemaIndexAssemblyMatchesFullAssembly(t *testing.T) {
	run := func(t *testing.T, args []string) (string, *recordingSnapshot) {
		t.Helper()
		snapshot := newRecordingSnapshot(t)
		opens := 0
		var out bytes.Buffer
		root, err := BuildForArgs(
			context.Background(),
			cmdutil.InvocationContext{},
			args,
			WithIO(strings.NewReader(""), &out, io.Discard),
			WithoutPlugins(),
			WithoutStrictMode(),
			withRecordingSnapshot(snapshot, &opens),
		)
		if err != nil {
			t.Fatalf("BuildForArgs(%q): %v", args, err)
		}
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("execute %q: %v", args, err)
		}
		return out.String(), snapshot
	}

	indexOut, indexSnapshot := run(t, []string{"schema"})
	fullOut, fullSnapshot := run(t, []string{"schema", "--format", "json"})

	if indexSnapshot.catalogCalls != 0 || indexSnapshot.fullCalls != 0 {
		t.Errorf("bare schema parsed shards: Catalog=%d FullCatalog=%d, want 0 and 0",
			indexSnapshot.catalogCalls, indexSnapshot.fullCalls)
	}
	if fullSnapshot.fullCalls != 1 {
		t.Fatalf("control used FullCatalog %d times, want 1 — it is no longer a full-assembly control",
			fullSnapshot.fullCalls)
	}
	if indexOut == "" {
		t.Fatal("bare schema produced no output")
	}
	if indexOut != fullOut {
		t.Errorf("index assembly output differs from full assembly\nindex:\n%s\nfull:\n%s", indexOut, fullOut)
	}
}

func TestBuildWithInvocationArgsUsesTargetAssemblyAndExecutionArgs(t *testing.T) {
	snapshot := newRecordingSnapshot(t)
	opens := 0
	var stdout bytes.Buffer
	args := []string{"drive", "files", "list", "--help"}

	root := Build(
		context.Background(),
		cmdutil.InvocationContext{},
		WithInvocationArgs(args),
		WithIO(strings.NewReader(""), &stdout, io.Discard),
		WithoutPlugins(),
		WithoutStrictMode(),
		withRecordingSnapshot(snapshot, &opens),
	)

	// WithInvocationArgs owns a defensive copy. Mutating the caller's slice
	// after Build must not change either the selected domain or Cobra dispatch.
	args[0] = "calendar"

	if findCommand(root, "drive files list") == nil {
		t.Fatal("target tree is missing drive files list")
	}
	if findCommand(root, "calendar") != nil {
		t.Fatal("target tree unexpectedly contains calendar")
	}
	if !reflect.DeepEqual(snapshot.catalogNames, [][]string{{"drive"}}) || snapshot.fullCalls != 0 {
		t.Fatalf("Catalog selection = target:%#v full:%d, want drive only", snapshot.catalogNames, snapshot.fullCalls)
	}
	if state, _ := root.Context().Value(executionStateKey{}).(*buildResult); state == nil {
		t.Fatal("target Build root is missing execution state")
	}

	if err := root.Execute(); err != nil {
		t.Fatalf("Build target Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), "lark-cli drive files list [flags]") {
		t.Fatalf("drive files list help was not executed:\n%s", stdout.String())
	}
}

func TestBuildWithoutInvocationArgsRemainsFullAssembly(t *testing.T) {
	snapshot := newRecordingSnapshot(t)
	opens := 0

	root := Build(
		context.Background(),
		cmdutil.InvocationContext{},
		quietBuildOptions(snapshot, &opens)...,
	)

	if snapshot.fullCalls != 1 || snapshot.catalogCalls != 0 {
		t.Fatalf("Catalog selection = target:%d full:%d, want one full load", snapshot.catalogCalls, snapshot.fullCalls)
	}
	for _, path := range []string{"drive files list", "calendar", "docs +fetch"} {
		if findCommand(root, path) == nil {
			t.Errorf("full Build is missing %q", path)
		}
	}
}

func TestBuildForArgsDriveCatalogAndShortcutCoexist(t *testing.T) {
	snapshot := newRecordingSnapshot(t)
	opens := 0
	root, err := BuildForArgs(
		context.Background(),
		cmdutil.InvocationContext{},
		[]string{"drive", "+search", "--query", "quarterly", "--dry-run"},
		quietBuildOptions(snapshot, &opens)...,
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}

	for _, path := range []string{"drive +search", "drive files list"} {
		if findCommand(root, path) == nil {
			t.Errorf("target tree is missing %q", path)
		}
	}
	if findCommand(root, "calendar") != nil {
		t.Error("target tree unexpectedly contains calendar")
	}
	if !reflect.DeepEqual(snapshot.catalogNames, [][]string{{"drive"}}) {
		t.Fatalf("Catalog calls = %#v, want drive only", snapshot.catalogNames)
	}
}

func TestBuildForArgsCatalogOnlyTargetMountsNoShortcuts(t *testing.T) {
	snapshot := newRecordingSnapshot(t)
	opens := 0
	root, err := BuildForArgs(
		context.Background(),
		cmdutil.InvocationContext{},
		[]string{"approval", "--help"},
		quietBuildOptions(snapshot, &opens)...,
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}

	if findCommand(root, "approval") == nil {
		t.Fatal("catalog target approval is missing")
	}
	for _, irrelevant := range []string{"docs", "drive", "calendar"} {
		if findCommand(root, irrelevant) != nil {
			t.Errorf("catalog-only target unexpectedly contains shortcut root %q", irrelevant)
		}
	}
}

func TestBuildForArgsDocsIsPureShortcutTarget(t *testing.T) {
	snapshot := newRecordingSnapshot(t)
	opens := 0
	root, err := BuildForArgs(
		context.Background(),
		cmdutil.InvocationContext{},
		[]string{"docs", "+fetch"},
		quietBuildOptions(snapshot, &opens)...,
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}
	if findCommand(root, "docs +fetch") == nil {
		t.Fatal("docs +fetch is missing")
	}
	if len(findCommand(root, "docs").Commands()) == 0 {
		t.Fatal("docs target has no shortcuts")
	}
	if !reflect.DeepEqual(snapshot.catalogNames, [][]string{nil}) {
		t.Fatalf("Catalog calls = %#v, want one empty selection", snapshot.catalogNames)
	}
}

func TestBuildForArgsTargetSchemaExecutes(t *testing.T) {
	snapshot := newRecordingSnapshot(t)
	opens := 0
	var stdout bytes.Buffer
	root, err := BuildForArgs(
		context.Background(),
		cmdutil.InvocationContext{},
		[]string{"schema", "drive.file.comments.list"},
		WithIO(strings.NewReader(""), &stdout, io.Discard),
		WithoutPlugins(),
		WithoutStrictMode(),
		withRecordingSnapshot(snapshot, &opens),
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}
	root.SetArgs([]string{"schema", "drive.file.comments.list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("target schema Execute: %v", err)
	}
	if !strings.Contains(stdout.String(), `"name": "drive file.comments list"`) {
		t.Fatalf("schema output does not identify the target: %q", stdout.String())
	}
	if !reflect.DeepEqual(snapshot.catalogNames, [][]string{{"drive"}}) {
		t.Fatalf("Catalog calls = %#v, want drive only", snapshot.catalogNames)
	}
	for _, irrelevant := range []string{"docs", "calendar", "im"} {
		if findCommand(root, irrelevant) != nil {
			t.Errorf("target schema unexpectedly contains shortcut root %q", irrelevant)
		}
	}
}

func TestBuildForArgsFullAssemblyStillMountsAllShortcuts(t *testing.T) {
	snapshot := newRecordingSnapshot(t)
	opens := 0
	root, err := BuildForArgs(
		context.Background(),
		cmdutil.InvocationContext{},
		[]string{"--help"},
		quietBuildOptions(snapshot, &opens)...,
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}

	for _, shortcutRoot := range []string{"docs", "drive", "calendar"} {
		if findCommand(root, shortcutRoot) == nil {
			t.Errorf("full assembly is missing shortcut root %q", shortcutRoot)
		}
	}
}

func TestBuildForArgsPluginUsesTargetAndFrozenSnapshot(t *testing.T) {
	tmpHome(t)
	platform.ResetForTesting()
	t.Cleanup(platform.ResetForTesting)
	plugin := &countingInstallPlugin{name: "frozen"}
	platform.Register(plugin)

	snapshot := newRecordingSnapshot(t)
	opens := 0
	root, err := BuildForArgs(
		context.Background(),
		buildInvocationForTest(t),
		[]string{"drive", "files", "list"},
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutStrictMode(),
		withRecordingSnapshot(snapshot, &opens),
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}
	if snapshot.fullCalls != 0 || !reflect.DeepEqual(snapshot.catalogNames, [][]string{{"drive"}}) {
		t.Fatalf("Catalog selection = full:%d target:%#v, want drive only", snapshot.fullCalls, snapshot.catalogNames)
	}
	if plugin.installs != 1 {
		t.Fatalf("plugin installs = %d, want 1", plugin.installs)
	}
	if findCommand(root, "calendar") != nil {
		t.Fatal("plugin target build unexpectedly contains calendar")
	}
}

func TestBuildForArgsPluginSelectorsAndRestrictUseTargetCatalog(t *testing.T) {
	tests := []struct {
		name    string
		install func(platform.Registrar)
		caps    platform.Capabilities
		assert  func(*testing.T, *cobra.Command)
	}{
		{
			name: "all observer",
			install: func(r platform.Registrar) {
				r.Observe(platform.Before, "all", platform.All(), func(context.Context, platform.Invocation) {})
			},
			caps: platform.Capabilities{FailurePolicy: platform.FailClosed},
			assert: func(t *testing.T, root *cobra.Command) {
				assertBeforeObserverMatchesDrive(t, root)
			},
		},
		{
			name: "domain observer",
			install: func(r platform.Registrar) {
				r.Observe(platform.Before, "drive", platform.ByDomain("drive"), func(context.Context, platform.Invocation) {})
			},
			caps: platform.Capabilities{FailurePolicy: platform.FailClosed},
			assert: func(t *testing.T, root *cobra.Command) {
				assertBeforeObserverMatchesDrive(t, root)
			},
		},
		{
			name: "restrict",
			install: func(r platform.Registrar) {
				r.Restrict(&platform.Rule{Name: "deny-drive", Deny: []string{"drive/**"}, AllowUnannotated: true})
			},
			caps: platform.Capabilities{Restricts: true, FailurePolicy: platform.FailClosed},
			assert: func(t *testing.T, root *cobra.Command) {
				driveList := findCommand(root, "drive files list")
				if driveList == nil {
					t.Fatal("target tree is missing drive files list")
				}
				if !driveList.Hidden {
					t.Fatal("Restrict plugin did not hide drive files list")
				}
				if got := driveList.Annotations[cmdpolicy.AnnotationDenialLayer]; got != cmdpolicy.LayerPolicy {
					t.Fatalf("denial layer = %q, want %q", got, cmdpolicy.LayerPolicy)
				}
				err := driveList.RunE(driveList, nil)
				var denied *platform.CommandDeniedError
				if !errors.As(err, &denied) {
					t.Fatalf("drive files list error = %T %v, want CommandDeniedError", err, err)
				}
				if denied.RuleName != "deny-drive" {
					t.Fatalf("denial rule = %q, want deny-drive", denied.RuleName)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpHome(t)
			platform.ResetForTesting()
			t.Cleanup(platform.ResetForTesting)
			plugin := &assemblyPlugin{name: strings.ReplaceAll(tt.name, " ", "-"), caps: tt.caps, install: tt.install}
			platform.Register(plugin)

			snapshot := newRecordingSnapshot(t)
			opens := 0
			root, err := BuildForArgs(
				context.Background(),
				buildInvocationForTest(t),
				[]string{"drive", "files", "list"},
				WithIO(strings.NewReader(""), io.Discard, io.Discard),
				WithoutStrictMode(),
				withRecordingSnapshot(snapshot, &opens),
			)
			if err != nil {
				t.Fatalf("BuildForArgs: %v", err)
			}
			if snapshot.fullCalls != 0 || !reflect.DeepEqual(snapshot.catalogNames, [][]string{{"drive"}}) {
				t.Fatalf("Catalog selection = full:%d target:%#v, want drive only", snapshot.fullCalls, snapshot.catalogNames)
			}
			if plugin.installs != 1 {
				t.Fatalf("plugin installs = %d, want 1", plugin.installs)
			}
			if findCommand(root, "drive") == nil {
				t.Fatal("target tree is missing drive")
			}
			if findCommand(root, "calendar") != nil {
				t.Fatal("target tree unexpectedly contains calendar")
			}
			tt.assert(t, root)
		})
	}
}

func assertBeforeObserverMatchesDrive(t *testing.T, root *cobra.Command) {
	t.Helper()
	driveList := findCommand(root, "drive files list")
	if driveList == nil {
		t.Fatal("target tree is missing drive files list")
	}
	state, _ := root.Context().Value(executionStateKey{}).(*buildResult)
	if state == nil || state.registry == nil {
		t.Fatal("plugin hook registry is missing")
	}
	matches := state.registry.MatchingObservers(cobraCommandViewSource{}.View(driveList), platform.Before)
	if len(matches) != 1 {
		t.Fatalf("Before observers matching drive files list = %d, want 1", len(matches))
	}
}

func TestBuildForArgsLatePluginRegistrationDoesNotChangeFrozenTarget(t *testing.T) {
	tmpHome(t)
	platform.ResetForTesting()
	t.Cleanup(platform.ResetForTesting)

	snapshot := newRecordingSnapshot(t)
	opens := 0
	late := &countingInstallPlugin{name: "late"}
	root, err := BuildForArgs(
		context.Background(),
		buildInvocationForTest(t),
		[]string{"drive", "files", "list"},
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutStrictMode(),
		withRecordingSnapshot(snapshot, &opens),
		func(cfg *buildConfig) {
			cfg.afterSnapshotOpen = func() {
				platform.Register(late)
			}
		},
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}
	if snapshot.fullCalls != 0 || !reflect.DeepEqual(snapshot.catalogNames, [][]string{{"drive"}}) {
		t.Fatalf("Catalog selection = full:%d target:%#v, want frozen drive target", snapshot.fullCalls, snapshot.catalogNames)
	}
	if late.installs != 0 {
		t.Fatalf("late plugin installs = %d, want 0 from frozen snapshot", late.installs)
	}
	if findCommand(root, "calendar") != nil {
		t.Fatal("late plugin registration unexpectedly changed the frozen target tree")
	}
}

func TestBuildForArgsVersionRunsPluginLifecycleWithoutCatalog(t *testing.T) {
	tmpHome(t)
	platform.ResetForTesting()
	t.Cleanup(platform.ResetForTesting)
	startups := 0
	plugin := &assemblyPlugin{
		name: "version-lifecycle",
		caps: platform.Capabilities{FailurePolicy: platform.FailClosed},
		install: func(r platform.Registrar) {
			r.On(platform.Startup, "start", func(context.Context, *platform.LifecycleContext) error {
				startups++
				return nil
			})
		},
	}
	platform.Register(plugin)

	opens := 0
	root, err := BuildForArgs(
		context.Background(),
		buildInvocationForTest(t),
		[]string{"--version"},
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutStrictMode(),
		func(cfg *buildConfig) {
			cfg.snapshotOpener = func() (catalogSnapshot, error) {
				opens++
				return nil, errors.New("version must not open catalog")
			}
		},
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}
	if opens != 0 {
		t.Fatalf("Snapshot opens = %d, want 0", opens)
	}
	if plugin.installs != 1 {
		t.Fatalf("plugin installs = %d, want 1", plugin.installs)
	}
	if startups != 1 {
		t.Fatalf("plugin startups = %d, want 1", startups)
	}
	if root == nil {
		t.Fatal("version root is nil")
	}
}

type countingInstallPlugin struct {
	name     string
	installs int
}

type assemblyPlugin struct {
	name     string
	caps     platform.Capabilities
	install  func(platform.Registrar)
	installs int
}

func (p *assemblyPlugin) Name() string                        { return p.name }
func (p *assemblyPlugin) Version() string                     { return "1.0.0" }
func (p *assemblyPlugin) Capabilities() platform.Capabilities { return p.caps }
func (p *assemblyPlugin) Install(r platform.Registrar) error {
	p.installs++
	p.install(r)
	return nil
}

func (p *countingInstallPlugin) Name() string    { return p.name }
func (p *countingInstallPlugin) Version() string { return "1.0.0" }
func (p *countingInstallPlugin) Capabilities() platform.Capabilities {
	return platform.Capabilities{FailurePolicy: platform.FailClosed}
}
func (p *countingInstallPlugin) Install(platform.Registrar) error {
	p.installs++
	return nil
}

func TestCatalogFailureContracts(t *testing.T) {
	cause := errors.New("broken embedded bytes")
	catalogErr := errs.NewInternalError(
		errs.SubtypeCatalogIntegrity,
		"embedded catalog manifest is invalid: invalid JSON",
	).WithCause(cause)
	failingOpener := func(cfg *buildConfig) {
		cfg.snapshotOpener = func() (catalogSnapshot, error) {
			return nil, catalogErr
		}
	}

	_, err := BuildForArgs(
		context.Background(),
		cmdutil.InvocationContext{},
		[]string{"drive"},
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutPlugins(),
		failingOpener,
	)
	if !errors.Is(err, cause) {
		t.Fatalf("BuildForArgs error = %v, want preserved cause", err)
	}
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Subtype != errs.SubtypeCatalogIntegrity {
		t.Fatalf("BuildForArgs problem = %#v, %v", problem, ok)
	}

	root := Build(
		context.Background(),
		cmdutil.InvocationContext{},
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutPlugins(),
		failingOpener,
	)
	if len(root.Commands()) != 0 {
		t.Fatalf("fail-closed root has %d partial commands, want 0", len(root.Commands()))
	}
	root.SetArgs([]string{"drive"})
	err = root.Execute()
	if !errors.Is(err, cause) || output.ExitCodeOf(err) != output.ExitInternal {
		t.Fatalf("guard error = %v, exit=%d", err, output.ExitCodeOf(err))
	}

	targetRoot := Build(
		context.Background(),
		cmdutil.InvocationContext{},
		WithInvocationArgs([]string{"drive"}),
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutPlugins(),
		failingOpener,
	)
	if len(targetRoot.Commands()) != 0 {
		t.Fatalf("target fail-closed root has %d partial commands, want 0", len(targetRoot.Commands()))
	}
	err = targetRoot.Execute()
	if !errors.Is(err, cause) || output.ExitCodeOf(err) != output.ExitInternal {
		t.Fatalf("target guard error = %v, exit=%d", err, output.ExitCodeOf(err))
	}
}

func TestVersionDoesNotOpenBrokenSnapshot(t *testing.T) {
	var stdout bytes.Buffer
	opens := 0
	root, err := BuildForArgs(
		context.Background(),
		cmdutil.InvocationContext{},
		[]string{"--version"},
		WithIO(strings.NewReader(""), &stdout, io.Discard),
		WithoutPlugins(),
		func(cfg *buildConfig) {
			cfg.snapshotOpener = func() (catalogSnapshot, error) {
				opens++
				return nil, errors.New("must not be opened")
			}
		},
	)
	if err != nil {
		t.Fatalf("BuildForArgs: %v", err)
	}
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version Execute: %v", err)
	}
	if opens != 0 {
		t.Fatalf("Snapshot opens = %d, want 0", opens)
	}
	if !strings.Contains(stdout.String(), "lark-cli version") {
		t.Fatalf("version output = %q", stdout.String())
	}
}

func TestFullTargetCommandContract(t *testing.T) {
	snapshot, err := registry.OpenSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := snapshot.FullCatalog()
	if err != nil {
		t.Fatal(err)
	}
	opts := []BuildOption{
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutPlugins(),
		WithoutStrictMode(),
		WithAPICatalog(catalog),
	}
	full := Build(context.Background(), cmdutil.InvocationContext{}, opts...)

	domains := append(catalogServiceNames(catalog), shortcuts.ShortcutServiceNames()...)
	sort.Strings(domains)
	domains = compactTestStrings(domains)
	for _, domain := range domains {
		t.Run(domain, func(t *testing.T) {
			want := findCommand(full, domain)
			if want == nil {
				t.Fatalf("full tree is missing targetable domain %q", domain)
			}
			target, err := BuildForArgs(context.Background(), cmdutil.InvocationContext{}, []string{domain}, opts...)
			if err != nil {
				t.Fatalf("BuildForArgs: %v", err)
			}
			got := findCommand(target, domain)
			if got == nil {
				t.Fatalf("target tree is missing domain %q", domain)
			}
			compareCommandTrees(t, want, got)
		})
	}
}

func TestFullTargetTypedValidationContract(t *testing.T) {
	snapshot, err := registry.OpenSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	catalog, err := snapshot.FullCatalog()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		args      []string
		wantParam string
	}{
		{
			name:      "catalog and shortcut validation",
			args:      []string{"drive", "+search", "--creator-ids", "not-an-open-id", "--dry-run", "--as", "bot"},
			wantParam: "--creator-ids",
		},
		{
			name:      "generated api required path flag",
			args:      []string{"drive", "files", "copy", "--dry-run", "--as", "bot"},
			wantParam: "file_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullErr := executeAssemblyForValidation(t, catalog, tt.args, false)
			targetErr := executeAssemblyForValidation(t, catalog, tt.args, true)
			fullContract := typedErrorContractOf(t, fullErr)
			targetContract := typedErrorContractOf(t, targetErr)
			if !reflect.DeepEqual(fullContract, targetContract) {
				t.Fatalf("Full error = %#v, Target error = %#v", fullContract, targetContract)
			}
			if fullContract.Category != errs.CategoryValidation ||
				fullContract.Subtype != errs.SubtypeInvalidArgument ||
				fullContract.Param != tt.wantParam ||
				fullContract.ExitCode != output.ExitValidation {
				t.Fatalf("validation contract = %#v", fullContract)
			}
		})
	}
}

func executeAssemblyForValidation(
	t *testing.T,
	catalog apicatalog.Catalog,
	args []string,
	target bool,
) error {
	t.Helper()
	t.Setenv("LARKSUITE_CLI_CONFIG_DIR", t.TempDir())
	saveAppsForTest(t, []core.AppConfig{{
		Name:      "default",
		AppId:     "cli_test",
		AppSecret: core.PlainSecret("test-secret"),
		Brand:     core.BrandLark,
	}})
	opts := []BuildOption{
		WithIO(strings.NewReader(""), io.Discard, io.Discard),
		WithoutPlugins(),
		WithoutStrictMode(),
		WithAPICatalog(catalog),
	}
	var root *cobra.Command
	if target {
		var err error
		root, err = BuildForArgs(context.Background(), cmdutil.InvocationContext{}, args, opts...)
		if err != nil {
			t.Fatalf("BuildForArgs: %v", err)
		}
	} else {
		root = Build(context.Background(), cmdutil.InvocationContext{}, opts...)
	}
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		t.Fatalf("%s unexpectedly succeeded", strings.Join(args, " "))
	}
	return err
}

type typedErrorContract struct {
	Category errs.Category
	Subtype  errs.Subtype
	Param    string
	ExitCode int
}

func typedErrorContractOf(t testing.TB, err error) typedErrorContract {
	t.Helper()
	problem, ok := errs.ProblemOf(err)
	if !ok {
		t.Fatalf("error is not typed: %T %v", err, err)
	}
	var validation *errs.ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error is not a ValidationError: %T %v", err, err)
	}
	return typedErrorContract{
		Category: problem.Category,
		Subtype:  problem.Subtype,
		Param:    validation.Param,
		ExitCode: output.ExitCodeOf(err),
	}
}

func compactTestStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := values[:1]
	for _, value := range values[1:] {
		if value != out[len(out)-1] {
			out = append(out, value)
		}
	}
	return out
}

type commandContract struct {
	Use                 string
	Aliases             []string
	Short               string
	Long                string
	Example             string
	Hidden              bool
	Deprecated          string
	DisableFlagParsing  bool
	TraverseChildren    bool
	Annotations         map[string]string
	Local               []flagContract
	Persistent          []flagContract
	Inherited           []flagContract
	Args                string
	PersistentPreRun    string
	PersistentPreRunE   string
	PreRun              string
	PreRunE             string
	Run                 string
	RunE                string
	PersistentHookChain []persistentHookContract
}

type persistentHookContract struct {
	CommandPath       string
	PersistentPreRun  string
	PersistentPreRunE string
}

type flagContract struct {
	Name        string
	Shorthand   string
	Usage       string
	Default     string
	NoOpt       string
	Hidden      bool
	Annotations map[string][]string
}

func compareCommandTrees(t *testing.T, want, got *cobra.Command) {
	t.Helper()
	if diff := compareContract(commandContractOf(want), commandContractOf(got)); diff != "" {
		t.Errorf("%s contract differs: %s", want.CommandPath(), diff)
	}
	wantChildren := commandChildren(want)
	gotChildren := commandChildren(got)
	if !reflect.DeepEqual(sortedKeys(wantChildren), sortedKeys(gotChildren)) {
		t.Fatalf("%s children = %v, want %v", want.CommandPath(), sortedKeys(gotChildren), sortedKeys(wantChildren))
	}
	for name, wantChild := range wantChildren {
		compareCommandTrees(t, wantChild, gotChildren[name])
	}
}

func commandContractOf(cmd *cobra.Command) commandContract {
	return commandContract{
		Use:                 cmd.Use,
		Aliases:             append([]string(nil), cmd.Aliases...),
		Short:               cmd.Short,
		Long:                cmd.Long,
		Example:             cmd.Example,
		Hidden:              cmd.Hidden,
		Deprecated:          cmd.Deprecated,
		DisableFlagParsing:  cmd.DisableFlagParsing,
		TraverseChildren:    cmd.TraverseChildren,
		Annotations:         cloneStringMap(cmd.Annotations),
		Local:               flagContracts(cmd.LocalNonPersistentFlags()),
		Persistent:          flagContracts(cmd.PersistentFlags()),
		Inherited:           flagContracts(cmd.InheritedFlags()),
		Args:                stableFunctionName(cmd.Args),
		PersistentPreRun:    stableFunctionName(cmd.PersistentPreRun),
		PersistentPreRunE:   stableFunctionName(cmd.PersistentPreRunE),
		PreRun:              stableFunctionName(cmd.PreRun),
		PreRunE:             stableFunctionName(cmd.PreRunE),
		Run:                 stableFunctionName(cmd.Run),
		RunE:                stableFunctionName(cmd.RunE),
		PersistentHookChain: persistentHookChain(cmd),
	}
}

func stableFunctionName(fn interface{}) string {
	if fn == nil {
		return ""
	}
	value := reflect.ValueOf(fn)
	if value.Kind() != reflect.Func || value.IsNil() {
		return ""
	}
	entry := runtime.FuncForPC(value.Pointer())
	if entry == nil {
		return ""
	}
	return entry.Name()
}

func persistentHookChain(cmd *cobra.Command) []persistentHookContract {
	var ancestry []*cobra.Command
	for current := cmd; current != nil; current = current.Parent() {
		ancestry = append(ancestry, current)
	}
	chain := make([]persistentHookContract, 0, len(ancestry))
	for i := len(ancestry) - 1; i >= 0; i-- {
		current := ancestry[i]
		preRun := stableFunctionName(current.PersistentPreRun)
		preRunE := stableFunctionName(current.PersistentPreRunE)
		if preRun == "" && preRunE == "" {
			continue
		}
		chain = append(chain, persistentHookContract{
			CommandPath:       current.CommandPath(),
			PersistentPreRun:  preRun,
			PersistentPreRunE: preRunE,
		})
	}
	return chain
}

func flagContracts(flags *pflag.FlagSet) []flagContract {
	var out []flagContract
	flags.VisitAll(func(flag *pflag.Flag) {
		out = append(out, flagContract{
			Name:        flag.Name,
			Shorthand:   flag.Shorthand,
			Usage:       flag.Usage,
			Default:     flag.DefValue,
			NoOpt:       flag.NoOptDefVal,
			Hidden:      flag.Hidden,
			Annotations: cloneStringSlices(flag.Annotations),
		})
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func compareContract(want, got commandContract) string {
	if reflect.DeepEqual(want, got) {
		return ""
	}
	return "metadata or flags changed"
}

func commandChildren(cmd *cobra.Command) map[string]*cobra.Command {
	children := make(map[string]*cobra.Command)
	for _, child := range cmd.Commands() {
		children[child.Name()] = child
	}
	return children
}

func sortedKeys(values map[string]*cobra.Command) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func cloneStringSlices(values map[string][]string) map[string][]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string][]string, len(values))
	for key, value := range values {
		out[key] = append([]string(nil), value...)
	}
	return out
}

// restrictDeclPlugin varies only the declaration the planner reads, including
// the case where reading it fails.
type restrictDeclPlugin struct {
	name   string
	caps   platform.Capabilities
	panics bool
}

func (p restrictDeclPlugin) Name() string    { return p.name }
func (p restrictDeclPlugin) Version() string { return "1.0.0" }
func (p restrictDeclPlugin) Capabilities() platform.Capabilities {
	if p.panics {
		panic("deliberate capabilities panic")
	}
	return p.caps
}
func (p restrictDeclPlugin) Install(platform.Registrar) error { return nil }

// TestAnyPluginRestricts covers the signal the schema index is gated on. The
// panic case is the one worth pinning: Capabilities is third-party code called
// ahead of the install pipeline that owns its failures, so planning must not
// turn a faulty plugin into a stack trace, nor read the missing answer as "no
// restriction".
func TestAnyPluginRestricts(t *testing.T) {
	t.Parallel()

	plain := restrictDeclPlugin{name: "plain"}
	policy := restrictDeclPlugin{name: "policy", caps: platform.Capabilities{Restricts: true}}
	faulty := restrictDeclPlugin{name: "faulty", panics: true}

	tests := []struct {
		name    string
		plugins []platform.Plugin
		want    bool
	}{
		{name: "no plugins at all", plugins: nil, want: false},
		{name: "none declares a restriction", plugins: []platform.Plugin{plain}, want: false},
		{name: "one declares a restriction", plugins: []platform.Plugin{plain, policy}, want: true},
		{name: "a panicking declaration counts as restricting", plugins: []platform.Plugin{faulty}, want: true},
		{name: "a panicking one does not hide a later declaration", plugins: []platform.Plugin{faulty, policy}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := anyPluginRestricts(tt.plugins); got != tt.want {
				t.Errorf("anyPluginRestricts = %v, want %v", got, tt.want)
			}
		})
	}
}
