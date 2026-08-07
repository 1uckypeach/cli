// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"io"
	"io/fs"
	"strings"
	"sync"

	"github.com/larksuite/cli/cmd/api"
	"github.com/larksuite/cli/cmd/auth"
	"github.com/larksuite/cli/cmd/completion"
	cmdconfig "github.com/larksuite/cli/cmd/config"
	"github.com/larksuite/cli/cmd/doctor"
	cmdevent "github.com/larksuite/cli/cmd/event"
	"github.com/larksuite/cli/cmd/profile"
	"github.com/larksuite/cli/cmd/schema"
	"github.com/larksuite/cli/cmd/service"
	"github.com/larksuite/cli/cmd/skill"
	cmdupdate "github.com/larksuite/cli/cmd/update"
	"github.com/larksuite/cli/cmd/whoami"
	"github.com/larksuite/cli/extension/platform"
	"github.com/larksuite/cli/internal/affordance"
	"github.com/larksuite/cli/internal/apicatalog"
	"github.com/larksuite/cli/internal/build"
	"github.com/larksuite/cli/internal/cmdpolicy"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/hook"
	"github.com/larksuite/cli/internal/keychain"
	"github.com/larksuite/cli/internal/meta"
	internalplatform "github.com/larksuite/cli/internal/platform"
	"github.com/larksuite/cli/internal/recovery"
	"github.com/larksuite/cli/internal/registry"
	"github.com/larksuite/cli/internal/skillpolicy"
	"github.com/larksuite/cli/internal/skillref"
	"github.com/larksuite/cli/internal/surface"
	"github.com/larksuite/cli/shortcuts"
	"github.com/spf13/cobra"
)

// BuildOption configures optional aspects of the command tree construction.
type BuildOption func(*buildConfig)

type buildConfig struct {
	streams           *cmdutil.IOStreams
	keychain          keychain.KeychainAccess
	globals           GlobalOptions
	invocationArgs    []string
	presentation      restrictionPresentationConfig
	skipPlugins       bool
	skipStrictMode    bool
	skipService       bool
	deferStartup      bool
	apiCatalog        *apicatalog.Catalog
	snapshotOpener    func() (catalogSnapshot, error)
	afterSnapshotOpen func()
	hideProfileSet    bool
}

type catalogSnapshot interface {
	ServiceNames() []string
	Catalog(names ...string) (apicatalog.Catalog, error)
	FullCatalog() (apicatalog.Catalog, error)
}

// buildRuntime owns presentation state for exactly one command tree. Factory
// remains the business dependency container; distribution policy never enters
// it. The embedded pointer preserves convenient access to Factory fields in
// cmd-internal tests without exposing the surface plan to business packages.
type buildRuntime struct {
	*cmdutil.Factory
	surface         *surface.Plan
	recovery        *recovery.Projector
	skillReferences *skillref.Resolver
}

// WithIO sets the IO streams for the CLI by wrapping raw reader/writers.
// Terminal detection is delegated to cmdutil.NewIOStreams.
func WithIO(in io.Reader, out, errOut io.Writer) BuildOption {
	return func(c *buildConfig) {
		c.streams = cmdutil.NewIOStreams(in, out, errOut)
	}
}

// WithKeychain sets the secret storage backend. If not provided, the platform keychain is used.
func WithKeychain(kc keychain.KeychainAccess) BuildOption {
	return func(c *buildConfig) {
		c.keychain = kc
	}
}

// embeddedSkillContent is the skill tree wired into cmdutil.Factory.SkillContent
// at build time. It is registered by the repo-root package main's init via
// SetEmbeddedSkillContent — it cannot be threaded through main.go without
// breaking the single-file preview build (see skills_embed.go). nil in builds
// that embed no skills; the `skills` commands then return a typed internal error.
var embeddedSkillContent fs.FS

// SetEmbeddedSkillContent registers the embedded skill tree. Called from the
// repo-root package main's init; a wrapper main can call it before Execute to
// supply its own skill content.
func SetEmbeddedSkillContent(fsys fs.FS) { embeddedSkillContent = fsys }

// SetEmbeddedAffordanceContent registers the per-domain command guidance tree.
// Wrapper mains should wire the repository's affordance directory alongside
// embedded skills so generic --help presentation remains complete and skill
// references follow the composed distribution.
func SetEmbeddedAffordanceContent(fsys fs.FS) { affordance.SetSource(fsys) }

// HideProfile sets the visibility policy for the root-level --profile flag.
// When hide is true the flag stays registered (so existing invocations still
// parse) but is omitted from help and shell completion. Typically called as
// HideProfile(isSingleAppMode()).
func HideProfile(hide bool) BuildOption {
	return func(c *buildConfig) {
		c.globals.HideProfile = hide
		c.hideProfileSet = true
	}
}

// WithInvocationArgs enables target-aware assembly for Build. The provided
// arguments are used both to select the Catalog and Shortcut domains and as
// Cobra's execution arguments, keeping assembly and dispatch on one source of
// truth. Build remains a full-tree constructor when this option is omitted.
//
// An explicitly empty slice represents a bare invocation and therefore still
// assembles the complete tree for root help. The slice is copied so later
// caller mutation cannot change the planned or executed command.
func WithInvocationArgs(args []string) BuildOption {
	return func(c *buildConfig) {
		c.invocationArgs = append(make([]string, 0, len(args)), args...)
	}
}

// WithoutPlugins builds only repository-owned commands. It is intended for
// inspection tools that need a deterministic command tree.
func WithoutPlugins() BuildOption {
	return func(c *buildConfig) {
		c.skipPlugins = true
	}
}

// WithoutStrictMode builds the complete repository-owned command tree without
// applying user/profile strict-mode pruning. It is intended for offline
// inspection tools, not production execution.
func WithoutStrictMode() BuildOption {
	return func(c *buildConfig) {
		c.skipStrictMode = true
	}
}

// WithoutServiceCommands builds only hand-authored commands. It is intended for
// repository quality gates that should not depend on the remote OpenAPI
// metadata command surface.
func WithoutServiceCommands() BuildOption {
	return func(c *buildConfig) {
		c.skipService = true
	}
}

// WithServiceCatalog is the compatibility spelling for WithAPICatalog.
func WithServiceCatalog(catalog apicatalog.Catalog) BuildOption {
	return WithAPICatalog(catalog)
}

// WithAPICatalog uses catalog as the authoritative metadata for the entire
// command build. It is primarily intended for deterministic inspection tools
// and tests.
func WithAPICatalog(catalog apicatalog.Catalog) BuildOption {
	return func(c *buildConfig) {
		c.apiCatalog = &catalog
	}
}

// Build constructs the full command tree by default. When
// WithInvocationArgs is provided, it constructs only the command domains that
// those arguments can reach and configures Cobra to execute the same arguments.
//
// Build also installs registered plugins and emits the Startup lifecycle event
// during assembly -- so Plugin.On(Startup) handlers run even if the returned
// command is never dispatched. The matching Shutdown event is only emitted by
// Execute; callers that bypass Execute will not see Shutdown fire.
//
// Returns only the cobra.Command; Factory and hook Registry are internal.
// Use Execute for the standard production entry point.
func Build(ctx context.Context, inv cmdutil.InvocationContext, opts ...BuildOption) *cobra.Command {
	cfg := resolveBuildConfig(opts)
	if cfg.invocationArgs != nil {
		result, err := buildForArgsWithConfig(ctx, inv, cfg.invocationArgs, cfg)
		if err != nil {
			root := newCatalogFailureRoot(ctx, cfg, err)
			root.SetArgs(cfg.invocationArgs)
			return root
		}
		root := attachExecutionState(result)
		root.SetArgs(append(make([]string, 0, len(cfg.invocationArgs)), cfg.invocationArgs...))
		return root
	}

	plugins := frozenPlugins(cfg)
	catalog, err := fullCatalog(cfg)
	if err != nil {
		return newCatalogFailureRoot(ctx, cfg, err)
	}
	_, rootCmd, _ := assembleInternal(ctx, inv, newBuildFactory(inv, cfg), catalog, nil, plugins, cfg)
	return rootCmd
}

// BuildForArgs constructs only the command domains that args can reach. Cobra
// remains responsible for parsing and executing args after assembly.
func BuildForArgs(
	ctx context.Context,
	inv cmdutil.InvocationContext,
	args []string,
	opts ...BuildOption,
) (*cobra.Command, error) {
	cfg := resolveBuildConfig(opts)
	result, err := buildForArgsWithConfig(ctx, inv, args, cfg)
	if err != nil {
		return nil, err
	}
	return attachExecutionState(result), nil
}

type executionStateKey struct{}

type buildResult struct {
	runtime  *buildRuntime
	root     *cobra.Command
	registry *hook.Registry
}

func buildForArgsWithConfig(
	ctx context.Context,
	inv cmdutil.InvocationContext,
	args []string,
	cfg *buildConfig,
) (*buildResult, error) {
	if cfg == nil {
		cfg = &buildConfig{}
	}
	if cfg.streams == nil {
		cfg.streams = cmdutil.SystemIO()
	}
	if cfg.snapshotOpener == nil {
		cfg.snapshotOpener = func() (catalogSnapshot, error) { return registry.OpenSnapshot() }
	}
	plugins := frozenPlugins(cfg)
	f := newBuildFactory(inv, cfg)
	// Resolved at most once, and only by the branch that needs it. Reusing the
	// Factory's resolver is required rather than convenient: it fails open the
	// same way pruneForStrictMode does downstream, so a second detection here
	// could make the listing and the pruned command tree disagree.
	strictMode := sync.OnceValue(func() core.StrictMode {
		return f.ResolveStrictMode(ctx)
	})

	// Version is the only deterministic no-Catalog invocation. Plugins are
	// still installed below, and their Startup hooks run against the
	// repository-owned root even though no Catalog commands are assembled.
	// This first pass cannot answer catalog or Shortcut membership yet, so it
	// only recognizes the branches that need neither.
	preliminary := PlanAssembly(args, nil, nil, nil)
	if preliminary.Mode == AssemblyNone {
		runtime, root, reg := assembleInternal(ctx, inv, f, apicatalog.Catalog{}, []string{}, plugins, cfg)
		return &buildResult{runtime: runtime, root: root, registry: reg}, nil
	}

	var (
		snapshot catalogSnapshot
		names    []string
	)
	if cfg.apiCatalog != nil {
		names = catalogServiceNames(*cfg.apiCatalog)
	} else {
		var err error
		snapshot, err = cfg.snapshotOpener()
		if err != nil {
			return nil, err
		}
		names = snapshot.ServiceNames()
	}
	if cfg.afterSnapshotOpen != nil {
		cfg.afterSnapshotOpen()
	}

	// Plugins can observe, wrap, restrict, and handle lifecycle events, but
	// cannot add commands. Their frozen snapshot therefore does not broaden
	// the built-in Catalog or Shortcut domains needed for this invocation.
	plan := PlanAssembly(args, names, shortcuts.ShortcutServiceNames(), strictMode)
	catalog, err := catalogForPlan(cfg, snapshot, plan, names)
	if err != nil {
		return nil, err
	}
	runtime, root, reg := assembleInternal(ctx, inv, f, catalog, plan.ShortcutDomains, plugins, cfg)
	return &buildResult{runtime: runtime, root: root, registry: reg}, nil
}

func attachExecutionState(result *buildResult) *cobra.Command {
	result.root.SetContext(context.WithValue(result.root.Context(), executionStateKey{}, result))
	return result.root
}

func resolveBuildConfig(opts []BuildOption) *buildConfig {
	cfg := &buildConfig{snapshotOpener: func() (catalogSnapshot, error) {
		return registry.OpenSnapshot()
	}}
	for _, o := range opts {
		if o != nil {
			o(cfg)
		}
	}
	if cfg.streams == nil {
		cfg.streams = cmdutil.SystemIO()
	}
	return cfg
}

// buildInternal is a pure assembly function: it wires the command tree from
// inv and BuildOptions alone. Any state-dependent decision (disk, network,
// env) belongs in the caller and must be threaded in via BuildOption.
//
// Returns (runtime, rootCmd, registry). The registry is nil when plugin
// install failed (FailClosed guard installed) or when no plugin produced
// hooks; callers that wire Shutdown emit must nil-check before calling
// hook.Emit.
func buildInternal(ctx context.Context, inv cmdutil.InvocationContext, opts ...BuildOption) (*buildRuntime, *cobra.Command, *hook.Registry) {
	return buildInternalWithConfig(ctx, inv, resolveBuildConfig(opts))
}

func frozenPlugins(cfg *buildConfig) []platform.Plugin {
	if cfg.skipPlugins {
		return nil
	}
	return platform.RegisteredPlugins()
}

// buildInternalWithConfig assembles the complete command tree from an
// already-applied option snapshot. Target-aware callers use
// buildForArgsWithConfig instead.
func buildInternalWithConfig(ctx context.Context, inv cmdutil.InvocationContext, cfg *buildConfig) (*buildRuntime, *cobra.Command, *hook.Registry) {
	if cfg == nil {
		cfg = resolveBuildConfig(nil)
	}
	if cfg.streams == nil {
		cfg.streams = cmdutil.SystemIO()
	}
	if cfg.snapshotOpener == nil {
		cfg.snapshotOpener = func() (catalogSnapshot, error) { return registry.OpenSnapshot() }
	}
	catalog, err := fullCatalog(cfg)
	if err != nil {
		root := newCatalogFailureRoot(ctx, cfg, err)
		f := cmdutil.NewDefault(cfg.streams, inv)
		runtime := &buildRuntime{Factory: f, surface: surface.NewPlan(nil)}
		runtime.recovery = recovery.NewProjector(func() *surface.Plan { return runtime.surface })
		f.Recovery = runtime.recovery
		return runtime, root, nil
	}
	return assembleInternal(ctx, inv, newBuildFactory(inv, cfg), catalog, nil, frozenPlugins(cfg), cfg)
}

func fullCatalog(cfg *buildConfig) (apicatalog.Catalog, error) {
	if cfg.apiCatalog != nil {
		return *cfg.apiCatalog, nil
	}
	snapshot, err := cfg.snapshotOpener()
	if err != nil {
		return apicatalog.Catalog{}, err
	}
	return snapshot.FullCatalog()
}

func catalogForPlan(
	cfg *buildConfig,
	snapshot catalogSnapshot,
	plan AssemblyPlan,
	names []string,
) (apicatalog.Catalog, error) {
	// The index selection names services without reading a shard, so it is
	// built from the manifest names in both the injected and snapshot paths.
	if plan.Mode == AssemblyIndex {
		return indexCatalog(names), nil
	}
	if cfg.apiCatalog != nil {
		if plan.Mode == AssemblyFull {
			return *cfg.apiCatalog, nil
		}
		return selectCatalog(*cfg.apiCatalog, plan.CatalogServices), nil
	}
	if plan.Mode == AssemblyFull {
		return snapshot.FullCatalog()
	}
	return snapshot.Catalog(plan.CatalogServices...)
}

// indexCatalog builds the name-only catalog behind AssemblyIndex. The services
// carry no ServicePath, which is what keeps them out of command registration —
// they exist to be listed, not to be called.
func indexCatalog(names []string) apicatalog.Catalog {
	services := make([]meta.Service, 0, len(names))
	for _, name := range names {
		services = append(services, meta.Service{Name: name})
	}
	return apicatalog.New(apicatalog.SourceEmbedded, services)
}

func catalogServiceNames(catalog apicatalog.Catalog) []string {
	names := make([]string, 0, len(catalog.Services()))
	for _, service := range catalog.Services() {
		names = append(names, service.Name)
	}
	return names
}

func selectCatalog(catalog apicatalog.Catalog, names []string) apicatalog.Catalog {
	selected := make(map[string]struct{}, len(names))
	for _, name := range names {
		selected[name] = struct{}{}
	}
	services := catalog.Services()
	filtered := services[:0:0]
	for _, service := range services {
		if _, ok := selected[service.Name]; ok {
			filtered = append(filtered, service)
		}
	}
	return apicatalog.New(catalog.Source(), filtered)
}

func newCatalogFailureRoot(ctx context.Context, cfg *buildConfig, catalogErr error) *cobra.Command {
	root := &cobra.Command{
		Use:           "lark-cli",
		Short:         "Lark/Feishu CLI — OAuth authorization, UAT management, API calls",
		Long:          rootLong,
		Version:       build.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return catalogErr
		},
	}
	root.SetContext(ctx)
	root.SetIn(cfg.streams.In)
	root.SetOut(cfg.streams.Out)
	root.SetErr(cfg.streams.ErrOut)
	root.DisableFlagParsing = true
	return root
}

// assembleInternal is a pure assembly function. It consumes the already selected
// Catalog, Shortcut domains, and frozen plugin snapshot; it never chooses or
// reloads any of them.
//
// Returns (runtime, rootCmd, registry). The registry is nil when plugin
// install failed (FailClosed guard installed) or when no plugin produced
// hooks; callers that wire Shutdown emit must nil-check before calling
// hook.Emit.
// newBuildFactory creates the Factory for one command build. It is separate
// from assembleInternal because the assembly plan may need to resolve strict
// mode before the catalog it selects exists, and NewDefault installs
// process-wide workspace and transport state that must happen exactly once.
func newBuildFactory(inv cmdutil.InvocationContext, cfg *buildConfig) *cmdutil.Factory {
	f := cmdutil.NewDefault(cfg.streams, inv)
	if cfg.keychain != nil {
		f.Keychain = cfg.keychain
	}
	f.SkillContent = embeddedSkillContent
	return f
}

func assembleInternal(
	ctx context.Context,
	inv cmdutil.InvocationContext,
	f *cmdutil.Factory,
	catalog apicatalog.Catalog,
	shortcutDomains []string,
	plugins []platform.Plugin,
	cfg *buildConfig,
) (*buildRuntime, *cobra.Command, *hook.Registry) {
	// cfg.globals.Profile is left zero here; it's bound to the --profile
	// flag in RegisterGlobalFlags and filled by cobra's parse step.

	// Reset the legacy process-global diagnostic snapshots before paths that
	// may return early. Distribution presentation state is deliberately not
	// stored here; it belongs to this build's immutable surface plan.
	cmdpolicy.SetActive(nil)
	internalplatform.SetActiveInventory(nil)

	f.APICatalog = catalog
	runtime := &buildRuntime{Factory: f}
	runtime.recovery = recovery.NewProjectorWithContext(func() *surface.Plan {
		return runtime.surface
	}, recovery.RenderContext{Profile: inv.Profile})
	f.Recovery = runtime.recovery
	rootCmd := &cobra.Command{
		Use:     "lark-cli",
		Short:   "Lark/Feishu CLI — OAuth authorization, UAT management, API calls",
		Long:    rootLong,
		Version: build.Version,
	}

	rootCmd.SetContext(ctx)
	rootCmd.SetIn(cfg.streams.In)
	rootCmd.SetOut(cfg.streams.Out)
	rootCmd.SetErr(cfg.streams.ErrOut)

	// Root-only usage template (curated Usage synopsis + skills footer); see
	// rootUsageTemplate.
	rootCmd.SetUsageTemplate(rootUsageTemplate)

	// Framework-generated skill pointers read this build's final content and
	// exact command surface lazily. A second Build therefore cannot rewrite
	// help rendered by the first tree.
	installTipsHelpFunc(rootCmd, catalog, func() fs.FS {
		if !runtime.surface.CanReference(surface.CommandSkillsRead) {
			return nil
		}
		return runtime.SkillContent
	}, func() *skillref.Resolver {
		return runtime.skillReferences
	}, runtime.recovery)
	rootCmd.SilenceErrors = true
	// SilenceUsage as a static field (not only in PersistentPreRun) so it also
	// covers flag-parse errors, which fail before PreRun runs — otherwise cobra
	// dumps usage instead of our structured error. SetFlagErrorFunc on root is
	// inherited by every subcommand, turning unknown-flag errors into a
	// structured "did you mean" envelope.
	rootCmd.SilenceUsage = true
	rootCmd.SetFlagErrorFunc(flagDidYouMean)

	RegisterGlobalFlags(rootCmd.PersistentFlags(), &cfg.globals)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		f.CurrentCommand = cmd
	}

	rootCmd.AddCommand(cmdconfig.NewCmdConfigWithRecovery(f, runtime.recovery))
	rootCmd.AddCommand(auth.NewCmdAuthWithRecovery(f, runtime.recovery))
	rootCmd.AddCommand(profile.NewCmdProfile(f))
	rootCmd.AddCommand(doctor.NewCmdDoctorWithRecovery(f, runtime.recovery))
	rootCmd.AddCommand(whoami.NewCmdWhoamiWithRecovery(f, runtime.recovery))
	rootCmd.AddCommand(api.NewCmdApiWithContext(ctx, f, nil))
	rootCmd.AddCommand(schema.NewCmdSchemaWithVisibility(f, func(path []string) bool {
		return runtime.surface.CanReference(surface.CommandID(strings.Join(path, "/")))
	}, nil))
	rootCmd.AddCommand(completion.NewCmdCompletion(f))
	rootCmd.AddCommand(cmdupdate.NewCmdUpdate(f))
	rootCmd.AddCommand(cmdevent.NewCmdEvents(f))
	rootCmd.AddCommand(skill.NewCmdSkill(f))
	if !cfg.skipService {
		service.RegisterServiceCommandsWithContext(ctx, rootCmd, f)
	}
	shortcuts.RegisterShortcutsForDomainsWithContext(ctx, rootCmd, f, shortcutDomains)

	classifyRootCommands(rootCmd)

	installUnknownSubcommandGuard(rootCmd)
	// Bare `lark-cli` in an interactive terminal offers an interactive upgrade
	// before printing help; non-bare invocations and non-TTY are unaffected.
	installRootUpgradePrompt(f, rootCmd, runtime.recovery)

	if mode := f.ResolveStrictMode(ctx); mode.IsActive() && !cfg.skipStrictMode {
		pruneForStrictMode(rootCmd, mode)
	}

	var (
		installResult *internalplatform.InstallResult
		pluginRules   []cmdpolicy.PluginRule
		pluginSkills  []skillpolicy.PluginSkill
		hookRegistry  *hook.Registry
		denied        map[string]cmdpolicy.Denial
	)

	if !cfg.skipPlugins {
		var installErr error
		installResult, installErr = installPluginsAndHooks(plugins, cfg.streams.ErrOut)
		if installErr != nil {
			installPluginInstallErrorGuard(rootCmd, installErr)
			return finalizeFailedBuild(runtime, rootCmd)
		}
		if installResult != nil {
			pluginRules = installResult.PluginRules
			pluginSkills = installResult.PluginSkills
			hookRegistry = installResult.Registry
		}

		// Policy errors fail-CLOSED when a plugin contributed (security
		// intent must not be silently dropped); yaml-only errors fail-OPEN
		// with a warning so a typo can't lock the user out.
		var policyErr error
		denied, policyErr = applyUserPolicyPruning(rootCmd, pluginRules)
		if policyErr != nil {
			if len(pluginRules) > 0 {
				installPluginConflictGuard(rootCmd, policyErr)
				return finalizeFailedBuild(runtime, rootCmd)
			}
			warnPolicyError(cfg.streams.ErrOut, policyErr)
		}
	}

	// Presentation is an explicit host projection over the exact enforcement
	// decisions. With no opt-in, legacy Restrict and YAML policy behavior is
	// mechanically unchanged.
	var hasConcealedCommands bool
	runtime.surface, hasConcealedCommands = applyDistributionPresentation(rootCmd, cfg.presentation, denied)

	// Resolve skill assets and canonical references before installing hooks.
	// A declared customization is a build-integrity boundary: failure must
	// happen before Startup so no lifecycle side effect is stranded.
	skillResolution, skillErr := skillpolicy.ResolveWithReferences(embeddedSkillContent, pluginSkills)
	if skillErr != nil {
		installPluginSkillErrorGuard(rootCmd, skillErr)
		return finalizeFailedBuild(runtime, rootCmd)
	}
	f.SkillContent = skillResolution.Content
	runtime.skillReferences = skillResolution.References
	f.SkillReferences = skillResolution.References

	// Global flags and their environment equivalents belong to the same
	// distribution capability. Flag tokens are rejected by applyPluginFlagGate;
	// install the equivalent guard for an environment-origin profile before
	// hooks, Startup, or business commands can observe the invocation.
	if installEnvironmentProfileGate(rootCmd, inv, runtime.surface) {
		recordInventory(installResult)
		return finalizeFailedBuild(runtime, rootCmd)
	}

	// Install hooks only on business commands. The concealment-specific help
	// command is attached afterwards, preserving Cobra's historical contract
	// that help is not observed or wrapped by plugins.
	if hookRegistry != nil {
		installHooks(rootCmd, hookRegistry)
	}
	if hasConcealedCommands {
		installHelpCommand(rootCmd)
	}
	finalizeRootCommandGroups(rootCmd, runtime.surface)

	if hookRegistry != nil && !cfg.deferStartup {
		if err := emitStartup(ctx, hookRegistry); err != nil {
			installPluginLifecycleErrorGuard(rootCmd, err)
			recordInventory(installResult)
			return runtime, rootCmd, nil
		}
	}

	recordInventory(installResult)
	return runtime, rootCmd, hookRegistry
}

func finalizeFailedBuild(runtime *buildRuntime, root *cobra.Command) (*buildRuntime, *cobra.Command, *hook.Registry) {
	finalizeRootCommandGroups(root, runtime.surface)
	return runtime, root, nil
}
