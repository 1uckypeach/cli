# Typed Shortcuts

Typed Shortcuts are the opt-in command framework built around `common.Define`.
They compile typed input, output, authorization, Help, and runtime behavior into
the existing `common.Shortcut` mount path.

This document is for maintainers adding or migrating a Shortcut. The Go types,
compiler checks, and tests remain the source of truth.

## Runtime boundary

Typed and Legacy Shortcuts intentionally coexist:

- a command created with `common.Define` uses the Typed compiler, binder, Help,
  runner, and result protocol;
- every other `common.Shortcut` continues to use the Legacy runner and Help;
- migrating one command must not change unrelated commands;
- the compiled Typed contract is private and is not registered in the public
  Meta API Schema catalog.

`common.Define` compiles a Definition during package initialization. Invalid
metadata, tags, shapes, aliases, relations, output declarations, or active
system-flag collisions panic with service, command, and field context. A broken
Definition must never fall back to a partially configured Legacy Shortcut.

## Minimal definition

```go
type detailArgs struct {
    ResourceID string `flag:"resource-id" schema:"required;minLength=1" doc:"resource ID"`
}

type detailData struct {
    ID    string `json:"id" schema:"required" doc:"resource ID"`
    Title string `json:"title" schema:"required" doc:"resource title"`
}

var Detail = common.Define(common.Definition[detailArgs, detailData]{
    Metadata: common.CommandMetadata{
        Service:     "example",
        Command:     "+detail",
        Description: "Get resource details",
        Risk:        common.RiskRead,
        Authorization: common.AuthorizationDefinition{
            Identities: map[common.Identity]common.IdentityAuthorization{
                common.IdentityUser: {
                    RequiredScopes: []string{"example:resource:read"},
                },
            },
        },
    },
    Hooks: common.Hooks[detailArgs, detailData]{
        Execute: func(ctx context.Context, command common.CommandContext, args *detailArgs) (common.Result[detailData], error) {
            data, err := common.DoTypedAPIJSON(ctx, command, http.MethodGet,
                "/open-apis/example/v1/resources/"+validate.EncodePathSegment(args.ResourceID), nil, nil)
            if err != nil {
                return common.Result[detailData]{}, err
            }
            return common.Success(detailData{
                ID: common.GetString(data, "id"),
                Title: common.GetString(data, "title"),
            }), nil
        },
    },
})
```

Keep request construction, business normalization, validation, and response
projection explicit. Do not move business semantics into the framework merely
to shorten a Definition.

## Migration invariants

A pure migration preserves the existing public behavior:

- command and flag spelling, aliases, defaults, and accepted encodings;
- required-flag and relation error timing;
- file and stdin resolution;
- identity and scope behavior;
- validation, DryRun, and confirmation order;
- HTTP method, URL, params, body, and fan-out order;
- stdout, stderr, format selection, jq behavior, exit status, and recovery hints.

New behavior requires an explicit opt-in and a test that directly asserts the
change. Characterize the Legacy command before replacing it; do not infer the
contract from names or prose alone.

Command-facing failures must remain typed `errs.*` errors. Pass through an
existing typed error unchanged. Preserve domain recovery projection with
`CommandContext.PresentError` when a command stores an error inside result data.

## Inputs

Input fields are declared by tags on `Args` and may be supplemented with
`InputDefinition.Fields` when aliases or a richer `ValueShape` are needed.

Use:

- `Provided[T]` when explicit presence differs from a zero value or default;
- `arg:"local"` for normalized state that is not a public flag;
- `arg:"inline"` to embed a generated flat input fragment;
- `CLIInput.ValueSources` for flag, file, and stdin support;
- `CLIInput.Encoding` for repeated, comma-or-repeated, or JSON values;
- `Relation` for complete, proven presence relationships.

Aliases must declare how their values combine or conflict. Do not silently
choose between different values. `InputResolvedFromSource` is available when a
Legacy guard must distinguish literal flag text from content already loaded
from a file or stdin.

The compiler validates the active mount plan rather than a speculative reserved
namespace. Framework flags such as `--as`, `--dry-run`, `--jq`, inherited
`--profile`, and Cobra `--help` cannot be reused. Conditional framework flags,
including high-risk `--yes` and complex-input schema flags, collide only when
that behavior is active. Existing business flags such as `--json`, `--format`,
non-high-risk `--yes`, and `--version` remain valid when the mount plan permits
them.

### Complex JSON inputs

Default Help summarizes large object and array inputs instead of recursively
printing every nested field. A public JSON field with a complete composite
`ValueShape` receives local introspection:

```text
lark-cli <service> <command> --print-schema
lark-cli <service> <command> --print-schema --flag-name <name>
```

Introspection runs before required flags, config, auth, input reads, hooks, and
network calls. Preserve an existing domain `PrintFlagSchema` callback when it is
more authoritative than the current `ValueShape`. Do not publish an incomplete
shape merely to obtain automatic introspection.

## Authorization

Declare a scope in `RequiredScopes` only when every execution path for that
identity requires it. The runner preflights required scopes.

Use `ConditionalScopes` when the requirement depends on parsed input, resource
type, action plan, remote state, or an optional enrichment. `When` and `Params`
are discovery facts, not executable predicates. Once domain code knows that a
conditional path will execute, check it at the behavior-compatible lifecycle
point:

```go
if ref.Kind == resourceWiki {
    if err := command.RequireConditionalScopes("wiki:node:read"); err != nil {
        return err
    }
}
```

Use `ScopeBestEffort` only when failure of that secondary operation does not
fail the primary command. If only an API response can determine the missing
scope, let the API enforce it rather than adding a speculative local preflight.
Server-provided `missing_scopes` remain authoritative.

Migration tests should assert required, conditional, and union scope sets for
each identity, plus both condition-selected and condition-not-selected paths.
Do not change DryRun or validation ordering for authorization uniformity.

## Hooks and runtime access

Hooks execute in the compiled lifecycle:

1. resolve inputs, bind Args, and check source-pre-run relations;
2. Normalize;
3. check after-prepare relations;
4. Validate;
5. DryRun or confirmation/Execute;
6. validate and emit the Result.

Use the restricted `CommandContext` rather than reaching into Legacy
`RuntimeContext`. It exposes identity, config, API client, FileIO, path helpers,
stderr, conditional-scope checking, source-state inspection, error presentation,
and the stderr-only `StartSpinner` lifecycle.

Hooks must not write command data to stdout. Return a `Result[Data]` and let the
runner emit it. Progress and warnings go to `CommandContext.Stderr()`. Finite
HTTP response bodies use `DoTypedAPIStream`; long-running stdout streaming is a
separate protocol and remains Legacy.

## Output

Choose the output mode that matches the Legacy command:

- zero-value `OutputGeneric` uses the framework JSON/table/CSV/NDJSON formatter
  and an optional `Renderers["pretty"]` callback;
- `OutputFixedJSON` preserves Legacy `Out`/`OutRaw`: accepted `--format`
  spellings remain compatible, but successful output is always a JSON envelope.

Only `pretty` may have a custom renderer. Table, CSV, and NDJSON are owned by
the framework formatter. A generic command without a pretty renderer preserves
the JSON fallback for `--format pretty`.

Set `DisableHTMLEscaping` only to preserve Legacy unescaped JSON envelopes. It
keeps literal `<`, `>`, and `&`; it does not enable bare output, bypass content
safety, or allow hooks to write stdout.

Use a struct for `Data` by default. `Data=any` is accepted only as a narrow
migration escape hatch when an established standard envelope already forwards
an arbitrary JSON value. Its private Schema projection is unconstrained JSON;
a nil interface preserves `omitempty` and therefore omits `data`. Do not use a
typed-nil map/slice when omission is part of the Legacy protocol.

### Result metadata

Typed Results may carry only the standard envelope metadata already supported
by the Emitter:

```go
Output: common.OutputDefinition{
    Meta: common.ResultMetaDefinition{Count: true, Pagination: true},
}

return common.Success(data).WithMeta(common.CountMeta(len(items))), nil
```

Use `PaginationResultMeta` with `*common.ResultPaginationMeta` for pagination.
The runner rejects undeclared metadata, negative count/items, pages below one,
and inconsistent completion/token pairs. `complete=true` has no next token;
an incomplete result must have one.

Do not expose `output.Meta` directly, add rollback metadata, or use an arbitrary
map. Metadata does not infer or validate a path inside business Data. Help and
the private Typed Schema project only the capabilities declared by the command.
Success and Partial both carry metadata in JSON/jq envelopes; Partial does not
gain a pretty protocol.

### Partial results

Declare `Output.Outcomes.PartialFailure` before returning `common.Partial`.
Use `FailedItems` for a truthful item-ledger path; leave it nil for a
result-level partial whose recovery data lives directly in `Data`. Standard
Typed partials emit an `ok:false` JSON/jq envelope and the declared non-zero
exit code.

Do not invent a failed-items array or point it at successful items solely to fit
the framework.

### File artifacts

`ArtifactDefinition` validates receipts already present in `Data`: path and,
when declared, media type and size. It does not write or stat files and does not
enforce overwrite policy. The command remains responsible for FileIO writes,
naming, overwrite/skip/rename behavior, and completing work before returning.

Use `Optional` only when a successful invocation legitimately produces no file.
Any present optional receipt is validated exactly like a required receipt.

## Handwritten definitions and code generation

Write `Args`, `Data`, and the Definition by hand by default. Consider generation
only when a domain already owns a maintained, machine-readable input
specification and handwritten fields would create a material second source of
truth.

A generator may transcribe deterministic input facts such as flag names, Go
types, required status, scalar defaults, enums, bounds, value sources, and
encodings. Keep these handwritten:

- metadata, identities, scopes, and risk;
- aliases and relation semantics;
- Normalize, Validate, DryRun, and Execute;
- output, renderers, partial outcomes, and artifacts;
- any transformation not explicitly represented by the canonical source.

Generated fragments use `arg:"inline"` and still pass through
`common.Define`; they do not bypass compiler checks. Generate only commands on
an explicit reviewed allowlist. The Sheets generator is the current example:
it keeps domain-owned complex JSON parsing and schema callbacks while producing
flat input fragments for selected migrations.

Expose generation through `//go:generate`, test canonical-source coverage
structurally, and require clean regeneration:

```bash
go generate ./shortcuts/<domain>/...
git diff --exit-code -- shortcuts/<domain>
```

## Migration checklist

1. Record the Legacy flags, aliases, presence rules, scopes, hook order, DryRun,
   execution request, output formats, stderr, errors, and exit behavior.
2. Define typed `Args` and `Data`; keep normalized-only state local.
3. Declare metadata and classify required versus conditional scopes per
   identity.
4. Preserve aliases, relations, input sources, complex-input discovery, and
   system-flag behavior.
5. Select the output mode matching the Legacy emitter and declare partial or
   artifact receipts only when they are truthful.
6. Implement hooks with typed errors and the restricted `CommandContext`.
7. Add direct structural tests for compiler output and business behavior.
8. Add the built-binary DryRun E2E required for Shortcut changes.
9. If generated input changed, regenerate and verify a clean tree.
10. Run the repository quality gates from `AGENTS.md`.

A migration test should fail if the implementation is reverted. Prefer direct
assertions on typed metadata, requests, results, and exit behavior over broad
message substrings or large snapshots.

## Current boundaries

Keep a command on the Legacy path when its established protocol requires a
Typed capability that is not implemented, including:

- long-running stdout streaming;
- bare text, JSON, XML, or binary output;
- custom named formats or non-JSON defaults;
- partial pretty output;
- returning both a Result and a second command error;
- artifact-based row filtering.

The audited hard protocol exemptions are currently:

- `base +record-get`, `base +record-list`, and `base +record-search`;
- `mail +triage` and `mail +watch`;
- `event +subscribe`;
- `slides +xml-get`;
- `base +table-copy`.

`base +table-copy-status` also remains Legacy because it writes status stderr
after stdout; migrating it without changing ordering would require a post-emit
hook. Do not grow the runner for these isolated cases.

Do not add declaration-only fields for unsupported behaviors. A new capability
needs a production consumer, a compiler contract, a runtime executor,
Help/Schema projection, and direct tests in the same change.

## Code references

- `typed_definition.go`: public Definition, inputs, authorization, hooks, and
  restricted command context.
- `typed_compiler.go` and `typed_compile_*.go`: registration-time compilation.
- `typed_binder.go`: invocation-time binding from the compiled plan.
- `typed_runner.go`: lifecycle and runtime dispatch.
- `typed_output.go` and `typed_result_protocol.go`: output and receipt contract.
- `typed_help.go` and `typed_help_render.go`: Typed Help facts and rendering.
- `typed_schema.go`: private compiled contract.
- `../sheets/internal/gen`: domain-owned generation example.
