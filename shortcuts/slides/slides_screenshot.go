// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/extension/fileio"
	"github.com/larksuite/cli/internal/util"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	defaultSlidesScreenshotDir = ".lark-slides/screenshots"
	maxSlidesPerScreenshot     = 10
)

var (
	unsafeScreenshotFileCharRegex = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
	slideNumberAliasRegex         = regexp.MustCompile(`^[0-9]+$`)
)

type slidesScreenshotFlagView interface {
	Str(string) string
	StrSlice(string) []string
	IntArray(string) []int
	Changed(string) bool
}

type slidesScreenshotFileRuntime interface {
	FileIO() fileio.FileIO
	ResolveSavePath(string) (string, error)
}

type slidesScreenshotRuntime interface {
	slidesScreenshotFlagView
	slidesScreenshotFileRuntime
}

type slidesScreenshotArgs struct {
	Content      common.Provided[string]   `flag:"content" schema:"optional" cli:"sources=flag|file|stdin" doc:"slide XML content to render directly"`
	Output       common.Provided[string]   `flag:"output" schema:"optional" doc:"preferred relative output path for one screenshot"`
	OutputDir    common.Provided[string]   `flag:"output-dir" schema:"optional;default=\".lark-slides/screenshots\"" doc:"relative directory for saved screenshots"`
	OutputName   common.Provided[string]   `flag:"output-name" schema:"optional" doc:"file name stem for content render output"`
	Presentation common.Provided[string]   `flag:"presentation" schema:"optional" doc:"xml_presentation_id, slides URL, or wiki URL"`
	Slide        common.Provided[string]   `flag:"slide" schema:"optional" doc:"compatibility selector routed by digits to slide-number, otherwise slide-id"`
	SlideID      common.Provided[[]string] `flag:"slide-id" schema:"optional" cli:"encoding=comma_or_repeated" doc:"slide page identifier; repeat or comma-separate up to 10"`
	SlideNumber  common.Provided[[]int]    `flag:"slide-number" schema:"optional" cli:"encoding=comma_or_repeated" doc:"slide page number; repeat or comma-separate up to 10"`

	RenderMode   bool            `arg:"local"`
	Ref          presentationRef `arg:"local"`
	SlideIDs     []string        `arg:"local"`
	SlideNumbers []int           `arg:"local"`
}

type slidesScreenshotItem struct {
	Format      string `json:"format" schema:"required;enum=png|jpeg" doc:"saved image format"`
	Path        string `json:"path" schema:"required;minLength=1" doc:"resolved local file path"`
	Size        int    `json:"size" schema:"required;minimum=0" doc:"saved file size in bytes"`
	SlideID     string `json:"slide_id" schema:"required" doc:"slide page ID when returned"`
	SlideNumber int    `json:"slide_number" schema:"required;minimum=0" doc:"slide page number when returned"`
}

type slidesScreenshotData struct {
	Output            string                 `json:"output,omitempty" schema:"optional" doc:"actual single output path"`
	OutputAdjusted    bool                   `json:"output_adjusted,omitempty" schema:"optional" doc:"whether the requested extension was adjusted"`
	OutputDir         string                 `json:"output_dir,omitempty" schema:"optional" doc:"requested output directory"`
	RequestedOutput   string                 `json:"requested_output,omitempty" schema:"optional" doc:"original requested output path when adjusted"`
	Screenshots       []slidesScreenshotItem `json:"screenshots" schema:"required;nonnullable" doc:"saved screenshot files"`
	XMLPresentationID string                 `json:"xml_presentation_id,omitempty" schema:"optional" doc:"resolved slides presentation ID"`
}

type typedSlidesScreenshotRuntime struct {
	args    *slidesScreenshotArgs
	command common.CommandContext
}

func (r typedSlidesScreenshotRuntime) Str(name string) string {
	switch name {
	case "content":
		return r.args.Content.Value
	case "output":
		return r.args.Output.Value
	case "output-dir":
		return r.args.OutputDir.Value
	case "output-name":
		return r.args.OutputName.Value
	case "presentation":
		return r.args.Presentation.Value
	case "slide":
		return r.args.Slide.Value
	}
	return ""
}
func (r typedSlidesScreenshotRuntime) StrSlice(name string) []string {
	if name == "slide-id" {
		return r.args.SlideID.Value
	}
	return nil
}
func (r typedSlidesScreenshotRuntime) IntArray(name string) []int {
	if name == "slide-number" {
		return r.args.SlideNumber.Value
	}
	return nil
}
func (r typedSlidesScreenshotRuntime) Changed(name string) bool {
	switch name {
	case "content":
		return r.args.Content.Set
	case "output":
		return r.args.Output.Set
	case "output-dir":
		return r.args.OutputDir.Set
	case "output-name":
		return r.args.OutputName.Set
	case "presentation":
		return r.args.Presentation.Set
	case "slide":
		return r.args.Slide.Set
	case "slide-id":
		return r.args.SlideID.Set
	case "slide-number":
		return r.args.SlideNumber.Set
	}
	return false
}
func (r typedSlidesScreenshotRuntime) FileIO() fileio.FileIO { return r.command.FileIO() }
func (r typedSlidesScreenshotRuntime) ResolveSavePath(path string) (string, error) {
	return r.command.ResolveSavePath(path)
}

func slidesScreenshotDataFromMap(result map[string]interface{}) slidesScreenshotData {
	data := slidesScreenshotData{
		Output: common.GetString(result, "output"), OutputAdjusted: common.GetBool(result, "output_adjusted"),
		OutputDir: common.GetString(result, "output_dir"), RequestedOutput: common.GetString(result, "requested_output"),
		XMLPresentationID: common.GetString(result, "xml_presentation_id"),
	}
	items, _ := result["screenshots"].([]map[string]interface{})
	for _, item := range items {
		data.Screenshots = append(data.Screenshots, slidesScreenshotItem{
			Format: common.GetString(item, "format"), Path: common.GetString(item, "path"), Size: common.GetInt(item, "size"),
			SlideID: common.GetString(item, "slide_id"), SlideNumber: common.GetInt(item, "slide_number"),
		})
	}
	return data
}

// SlidesScreenshot fetches server-rendered slide screenshots and writes them to
// local files. The raw API returns Base64 image payloads; this shortcut keeps
// those payloads out of stdout so agents only see small file metadata.
var SlidesScreenshot = common.Define(common.Definition[slidesScreenshotArgs, slidesScreenshotData]{
	Metadata: common.CommandMetadata{
		Service: "slides", Command: "+screenshot", Description: "Save up to 10 slide screenshots to local files without printing Base64 image data", Risk: common.RiskRead,
		Authorization: common.AuthorizationDefinition{Identities: map[common.Identity]common.IdentityAuthorization{
			common.IdentityUser: {RequiredScopes: []string{"slides:presentation:screenshot"}, ConditionalScopes: []common.ConditionalScope{{Scopes: []string{"wiki:node:read"}, When: "--presentation resolves to a Wiki node", Params: []string{"presentation"}}}},
			common.IdentityBot:  {RequiredScopes: []string{"slides:presentation:screenshot"}, ConditionalScopes: []common.ConditionalScope{{Scopes: []string{"wiki:node:read"}, When: "--presentation resolves to a Wiki node", Params: []string{"presentation"}}}},
		}},
	},
	Input: common.InputDefinition{Fields: []common.InputField{
		{Name: "presentation", CLI: common.CLIInput{Aliases: []common.FlagAlias{
			{Name: "presentation-id", Mode: common.AliasNormalize}, {Name: "presentation-token", Mode: common.AliasNormalize}, {Name: "token", Mode: common.AliasNormalize},
			{Name: "presentation_id", Mode: common.AliasNormalize}, {Name: "xml-presentation-id", Mode: common.AliasNormalize}, {Name: "url", Mode: common.AliasNormalize},
		}}},
		{Name: "slide", CLI: common.CLIInput{Hidden: true}},
		{Name: "slide-id", CLI: common.CLIInput{Aliases: []common.FlagAlias{{Name: "slide-ids", Mode: common.AliasNormalize}, {Name: "slides", Mode: common.AliasNormalize}}}},
		{Name: "slide-number", CLI: common.CLIInput{Aliases: []common.FlagAlias{{Name: "slide-numbers", Mode: common.AliasNormalize}}}},
	}},
	Output: common.OutputDefinition{
		Artifacts: []common.ArtifactDefinition{{Name: "screenshots", ItemsPath: "/screenshots", PathField: "/path", SizeField: "/size"}},
		Mode:      common.OutputFixedJSON,
	},
	Hooks: common.Hooks[slidesScreenshotArgs, slidesScreenshotData]{
		Normalize: func(_ context.Context, command common.CommandContext, args *slidesScreenshotArgs) error {
			runtime := typedSlidesScreenshotRuntime{args: args, command: command}
			args.RenderMode = args.Content.Set
			selectorCount := 1
			if args.RenderMode {
				if strings.TrimSpace(args.Content.Value) == "" {
					return slidesScreenshotFlagErrorf("--content cannot be empty")
				}
				if slidesScreenshotHasSelectorInput(runtime) {
					return slidesScreenshotContentSelectorConflictError(runtime)
				}
				if args.Presentation.Set {
					return slidesScreenshotFlagErrorf("--presentation cannot be used with --content")
				}
			} else {
				ref, err := parsePresentationRef(args.Presentation.Value)
				if err != nil {
					return err
				}
				ids, numbers, err := slidesScreenshotSelectors(runtime)
				if err != nil {
					return err
				}
				if ref.Kind == "wiki" {
					if err := command.RequireConditionalScopes("wiki:node:read"); err != nil {
						return err
					}
				}
				if len(ids) == 0 && len(numbers) == 0 {
					return slidesScreenshotMissingSelectorError()
				}
				selectorCount = len(ids) + len(numbers)
				if err := validateSlidesScreenshotSelectorLimit(selectorCount); err != nil {
					return err
				}
				args.Ref, args.SlideIDs, args.SlideNumbers = ref, ids, numbers
			}
			if args.Output.Set {
				if args.OutputDir.Set {
					return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output cannot be combined with --output-dir").WithParam("--output")
				}
				if args.OutputName.Set {
					return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output cannot be combined with --output-name").WithParam("--output")
				}
				if selectorCount != 1 {
					return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output requires exactly one slide; use --output-dir for multiple screenshots").WithParam("--output")
				}
				if err := validateScreenshotOutputPath(runtime, args.Output.Value); err != nil {
					return err
				}
			} else {
				if !args.RenderMode && args.OutputName.Set {
					return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output-name is only supported with --content").WithParam("--output-name").WithHint("use --output <file> for one existing slide, or --output-dir for multiple slides")
				}
				if _, err := validateScreenshotOutputDir(runtime, args.OutputDir.Value); err != nil {
					return err
				}
			}
			return nil
		},
		DryRun: func(_ context.Context, command common.CommandContext, args *slidesScreenshotArgs) *common.DryRunAPI {
			runtime := typedSlidesScreenshotRuntime{args: args, command: command}
			if args.RenderMode {
				return dryRunRenderScreenshot(runtime)
			}
			presentationID := args.Ref.Token
			dry := common.NewDryRunAPI()
			if args.Ref.Kind == "wiki" {
				presentationID = "<resolved_slides_token>"
				dry.Desc("2-step orchestration: resolve wiki → fetch slide screenshot(s)").GET("/open-apis/wiki/v2/spaces/get_node").Desc("[1] Resolve wiki node to slides presentation").Params(map[string]interface{}{"token": args.Ref.Token})
			} else if args.Output.Value != "" {
				dry.Desc(fmt.Sprintf("Fetch one slide screenshot and save it as %s", args.Output.Value))
			} else {
				dry.Desc(fmt.Sprintf("Fetch %d slide screenshot(s) and save files under %s", len(args.SlideIDs)+len(args.SlideNumbers), args.OutputDir.Value))
			}
			body := map[string]interface{}{}
			if len(args.SlideIDs) > 0 {
				body["slide_ids"] = args.SlideIDs
			}
			if len(args.SlideNumbers) > 0 {
				body["slide_numbers"] = args.SlideNumbers
			}
			dry.POST(fmt.Sprintf("/open-apis/slides_ai/v1/xml_presentations/%s/slide_images", validate.EncodePathSegment(presentationID))).Body(body)
			return setSlidesScreenshotDryRunOutput(dry, runtime).Set("base64_output", "suppressed; decoded to local files during execution")
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *slidesScreenshotArgs) (common.Result[slidesScreenshotData], error) {
			runtime := typedSlidesScreenshotRuntime{args: args, command: command}
			outputTarget, err := resolveSlidesScreenshotOutputTarget(runtime)
			if err != nil {
				return common.Result[slidesScreenshotData]{}, err
			}
			result := map[string]interface{}{}
			if args.RenderMode {
				data, err := common.DoTypedAPIJSON(ctx, command, "POST", "/open-apis/slides_ai/v1/slide_image/render", larkcore.QueryParams{}, map[string]interface{}{"content": args.Content.Value})
				if err != nil {
					return common.Result[slidesScreenshotData]{}, err
				}
				saved, err := saveRenderedSlideScreenshot(runtime, data, outputTarget.safeOutputDir, args.OutputName.Value, outputTarget.requested)
				if err != nil {
					return common.Result[slidesScreenshotData]{}, err
				}
				result["screenshots"] = saved
				setSlidesScreenshotResultOutput(result, outputTarget, saved)
				return common.Success(slidesScreenshotDataFromMap(result)), nil
			}
			presentationID, err := resolvePresentationIDTyped(ctx, command, args.Ref)
			if err != nil {
				return common.Result[slidesScreenshotData]{}, err
			}
			body := map[string]interface{}{}
			if len(args.SlideIDs) > 0 {
				body["slide_ids"] = args.SlideIDs
			}
			if len(args.SlideNumbers) > 0 {
				body["slide_numbers"] = args.SlideNumbers
			}
			apiPath := fmt.Sprintf("/open-apis/slides_ai/v1/xml_presentations/%s/slide_images", validate.EncodePathSegment(presentationID))
			data, err := common.DoTypedAPIJSON(ctx, command, "POST", apiPath, larkcore.QueryParams{}, body)
			if err != nil {
				return common.Result[slidesScreenshotData]{}, enrichSlidesScreenshotSelectorError(err, args.SlideNumbers)
			}
			saved, err := saveSlideScreenshots(runtime, data, outputTarget.safeOutputDir, presentationID, outputTarget.requested)
			if err != nil {
				return common.Result[slidesScreenshotData]{}, err
			}
			result["xml_presentation_id"], result["screenshots"] = presentationID, saved
			setSlidesScreenshotResultOutput(result, outputTarget, saved)
			return common.Success(slidesScreenshotDataFromMap(result)), nil
		},
	},
})

func dryRunRenderScreenshot(runtime slidesScreenshotFlagView) *common.DryRunAPI {
	content := runtime.Str("content")
	if strings.TrimSpace(content) == "" {
		return common.NewDryRunAPI().Set("error", "--content cannot be empty")
	}
	if slidesScreenshotHasSelectorInput(runtime) {
		return common.NewDryRunAPI().Set("error", "--content cannot be used with slide selectors")
	}
	if runtime.Changed("presentation") {
		return common.NewDryRunAPI().Set("error", "--presentation cannot be used with --content")
	}
	dry := common.NewDryRunAPI().Desc("Render slide XML content to a screenshot file")
	dry.POST("/open-apis/slides_ai/v1/slide_image/render").
		Body(map[string]interface{}{
			"content": fmt.Sprintf("<xml omitted; length=%d>", len(content)),
		})
	return setSlidesScreenshotDryRunOutput(dry, runtime).Set("base64_output", "suppressed; decoded to local file during execution")
}

func setSlidesScreenshotDryRunOutput(dry *common.DryRunAPI, runtime slidesScreenshotFlagView) *common.DryRunAPI {
	if outputPath := runtime.Str("output"); outputPath != "" {
		return dry.Set("output", outputPath)
	}
	return dry.Set("output_dir", runtime.Str("output-dir"))
}

type slidesScreenshotOutputTarget struct {
	requested         string
	requestedResolved string
	outputDir         string
	safeOutputDir     string
}

func resolveSlidesScreenshotOutputTarget(runtime slidesScreenshotRuntime) (slidesScreenshotOutputTarget, error) {
	target := slidesScreenshotOutputTarget{
		requested: runtime.Str("output"),
		outputDir: runtime.Str("output-dir"),
	}
	if target.requested != "" {
		resolved, err := runtime.ResolveSavePath(target.requested)
		if err != nil {
			return target, errs.NewValidationError(errs.SubtypeInvalidArgument, "--output invalid: %v", err).
				WithParam("--output").
				WithCause(err)
		}
		target.requestedResolved = resolved
		return target, nil
	}
	safeOutputDir, err := ensureScreenshotOutputDir(runtime, target.outputDir)
	if err != nil {
		return target, err
	}
	target.safeOutputDir = safeOutputDir
	return target, nil
}

func setSlidesScreenshotResultOutput(result map[string]interface{}, target slidesScreenshotOutputTarget, saved []map[string]interface{}) {
	if target.requested != "" && len(saved) == 1 {
		actualPath, _ := saved[0]["path"].(string)
		result["output"] = actualPath
		if filepath.Clean(target.requestedResolved) != filepath.Clean(actualPath) {
			result["requested_output"] = target.requested
			result["output_adjusted"] = true
		}
		return
	}
	result["output_dir"] = target.outputDir
}

func slidesScreenshotSelectors(runtime slidesScreenshotFlagView) ([]string, []int, error) {
	aliasSlide := strings.TrimSpace(runtime.Str("slide"))
	aliasSlideIsNumber := runtime.Changed("slide") && slideNumberAliasRegex.MatchString(aliasSlide)
	aliasSlideIsID := runtime.Changed("slide") && aliasSlide != "" && !aliasSlideIsNumber
	if runtime.Changed("slide") && aliasSlide == "" {
		return nil, nil, errs.NewValidationError(
			errs.SubtypeInvalidArgument,
			"--slide cannot be empty",
		).WithParam("--slide").WithHint("use --slide-number or --slide-id to specify the selector type explicitly")
	}

	slideIDValues := append([]string(nil), runtime.StrSlice("slide-id")...)
	if runtime.Changed("slide-id") && len(normalizeSlideIDs(slideIDValues)) == 0 {
		return nil, nil, slidesScreenshotEmptySlideIDError()
	}
	if aliasSlideIsID {
		slideIDValues = append(slideIDValues, aliasSlide)
	}

	slideNumberValues := append([]int(nil), runtime.IntArray("slide-number")...)
	if aliasSlideIsNumber {
		n, err := strconv.Atoi(aliasSlide)
		if err != nil {
			return nil, nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--slide page number is outside the supported integer range").WithParam("--slide")
		}
		if n < 1 {
			return nil, nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "--slide must be a positive page number").WithParam("--slide")
		}
		slideNumberValues = append(slideNumberValues, n)
	}

	slideNumbers, err := normalizeSlideNumbers(slideNumberValues)
	if err != nil {
		return nil, nil, err
	}
	slideIDs := normalizeSlideIDs(slideIDValues)
	if len(slideIDs) > 0 && len(slideNumbers) > 0 {
		return nil, nil, errs.NewValidationError(errs.SubtypeInvalidArgument, "slide ID selectors and slide number selectors cannot be used together").
			WithParams(slidesScreenshotSelectorConflictParams(runtime, aliasSlideIsID, aliasSlideIsNumber)...).
			WithHint("choose either slide IDs or slide numbers for one screenshot request")
	}
	return slideIDs, slideNumbers, nil
}

func slidesScreenshotHasSelectorInput(runtime slidesScreenshotFlagView) bool {
	return len(slidesScreenshotSelectorInputParams(runtime, "")) > 0
}

func slidesScreenshotSelectorConflictParams(runtime slidesScreenshotFlagView, aliasSlideIsID, aliasSlideIsNumber bool) []errs.InvalidParam {
	params := make([]errs.InvalidParam, 0, 3)
	if len(normalizeSlideIDs(runtime.StrSlice("slide-id"))) > 0 {
		params = append(params, errs.InvalidParam{Name: "--slide-id", Reason: "selects by slide ID; cannot be combined with slide-number selectors"})
	}
	if aliasSlideIsID {
		params = append(params, errs.InvalidParam{Name: "--slide", Reason: "selects by slide ID; cannot be combined with slide-number selectors"})
	}
	if runtime.Changed("slide-number") {
		params = append(params, errs.InvalidParam{Name: "--slide-number", Reason: "selects by slide number; cannot be combined with slide-ID selectors"})
	}
	if aliasSlideIsNumber {
		params = append(params, errs.InvalidParam{Name: "--slide", Reason: "selects by slide number; cannot be combined with slide-ID selectors"})
	}
	return params
}

func slidesScreenshotSelectorInputParams(runtime slidesScreenshotFlagView, reason string) []errs.InvalidParam {
	params := make([]errs.InvalidParam, 0, 3)
	if len(normalizeSlideIDs(runtime.StrSlice("slide-id"))) > 0 {
		params = append(params, errs.InvalidParam{Name: "--slide-id", Reason: reason})
	}
	if runtime.Changed("slide-number") {
		params = append(params, errs.InvalidParam{Name: "--slide-number", Reason: reason})
	}
	if runtime.Changed("slide") {
		params = append(params, errs.InvalidParam{Name: "--slide", Reason: reason})
	}
	return params
}

func slidesScreenshotContentSelectorConflictError(runtime slidesScreenshotFlagView) error {
	params := []errs.InvalidParam{{Name: "--content", Reason: "cannot be combined with slide selectors"}}
	params = append(params, slidesScreenshotSelectorInputParams(runtime, "cannot be combined with --content")...)
	return errs.NewValidationError(errs.SubtypeInvalidArgument, "--content cannot be used with slide selectors").WithParams(params...)
}

func normalizeSlideIDs(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, v := range values {
		s := strings.TrimSpace(v)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

func normalizeSlideNumbers(values []int) ([]int, error) {
	out := make([]int, 0, len(values))
	seen := map[int]struct{}{}
	for _, n := range values {
		if n < 1 {
			return nil, slidesScreenshotFlagErrorf("--slide-number must be a positive integer")
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out, nil
}

func validateSlidesScreenshotSelectorLimit(count int) error {
	if count > maxSlidesPerScreenshot {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "too many slide selectors: got %d, maximum is %d", count, maxSlidesPerScreenshot).
			WithHint("request at most 10 pages at a time")
	}
	return nil
}

func slidesScreenshotMissingSelectorError() error {
	return errs.NewValidationError(errs.SubtypeInvalidArgument, "--slide-id or --slide-number is required").
		WithHint("specify up to 10 slides with --slide-id <slide_id> or --slide-number <number>; repeat the flag or use comma-separated values for multiple slides")
}

func slidesScreenshotEmptySlideIDError() error {
	return errs.NewValidationError(errs.SubtypeInvalidArgument, "--slide-id cannot be empty").
		WithParam("--slide-id").
		WithHint("provide a non-empty slide ID or use --slide-number <number>")
}

func slidesScreenshotFlagErrorf(format string, args ...interface{}) error {
	return errs.NewValidationError(errs.SubtypeInvalidArgument, format, args...)
}

func validateScreenshotOutputDir(runtime slidesScreenshotFileRuntime, outputDir string) (string, error) {
	if _, err := runtime.ResolveSavePath(filepath.Join(outputDir, "probe.png")); err != nil {
		return "", slidesScreenshotFlagErrorf("--output-dir invalid: %v", err)
	}
	return outputDir, nil
}

func validateScreenshotOutputPath(runtime slidesScreenshotFileRuntime, outputPath string) error {
	if outputPath == "" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output cannot be empty").WithParam("--output")
	}
	if strings.TrimSpace(outputPath) != outputPath {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output cannot have leading or trailing whitespace").
			WithParam("--output").
			WithHint("remove surrounding whitespace and retry")
	}
	if os.IsPathSeparator(outputPath[len(outputPath)-1]) {
		return screenshotOutputDirectoryError(outputPath)
	}
	ext := strings.ToLower(filepath.Ext(outputPath))
	if ext != "" && ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output must have no extension or end with .png, .jpg, or .jpeg").WithParam("--output")
	}
	if _, err := runtime.ResolveSavePath(outputPath); err != nil {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output invalid: %v", err).WithParam("--output").WithCause(err)
	}
	if info, err := runtime.FileIO().Stat(outputPath); err == nil {
		if info.IsDir() {
			return screenshotOutputDirectoryError(outputPath)
		}
	} else if !isScreenshotFileNotExist(err) {
		return errs.NewInternalError(errs.SubtypeFileIO, "inspect --output path %s: %v", outputPath, err).WithCause(err)
	}
	return nil
}

func screenshotOutputDirectoryError(outputPath string) error {
	return errs.NewValidationError(errs.SubtypeInvalidArgument, "--output expects a file path, got directory %q", outputPath).
		WithParam("--output").
		WithHint("use --output-dir <directory> for directory output")
}

func ensureScreenshotOutputDir(runtime slidesScreenshotFileRuntime, outputDir string) (string, error) {
	return validateScreenshotOutputDir(runtime, outputDir)
}

func saveSlideScreenshots(runtime slidesScreenshotFileRuntime, data map[string]interface{}, outputDir string, presentationID string, outputPath string) ([]map[string]interface{}, error) {
	items := common.GetSlice(data, "slide_images")
	if len(items) == 0 {
		return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned no slide_images")
	}
	if outputPath != "" && len(items) != 1 {
		return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned %d slide_images for single --output", len(items))
	}
	saved := make([]map[string]interface{}, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned invalid slide_images[%d]", i)
		}
		item, err := saveSlideScreenshotImage(runtime, m, outputDir, slideScreenshotListFileBase(presentationID, m, i), "", outputPath)
		if err != nil {
			if isSlidesScreenshotPassthroughError(err) {
				return nil, err
			}
			return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned invalid slide_images[%d]: %v", i, err)
		}
		saved = append(saved, item)
	}
	return saved, nil
}

func saveRenderedSlideScreenshot(runtime slidesScreenshotFileRuntime, data map[string]interface{}, outputDir string, outputName string, outputPath string) ([]map[string]interface{}, error) {
	item := common.GetMap(data, "slide_image")
	if item == nil {
		return nil, slidesScreenshotAPIDataError(data, "slides render screenshot returned no slide_image")
	}
	saved, err := saveSlideScreenshotImage(runtime, item, outputDir, outputName, "rendered-slide", outputPath)
	if err != nil {
		if isSlidesScreenshotPassthroughError(err) {
			return nil, err
		}
		return nil, slidesScreenshotAPIDataError(data, "slides render screenshot returned invalid slide_image: %v", err)
	}
	return []map[string]interface{}{saved}, nil
}

func saveSlideScreenshotImage(runtime slidesScreenshotFileRuntime, item map[string]interface{}, outputDir string, outputName string, fallbackName string, outputPath string) (map[string]interface{}, error) {
	slideID := strings.TrimSpace(common.GetString(item, "slide_id"))
	ext, label, err := slideScreenshotFormat(item)
	if err != nil {
		return nil, slidesScreenshotImageDataError(slideID, "%s", err)
	}
	encoded := strings.TrimSpace(common.GetString(item, "data"))
	if encoded == "" {
		return nil, slidesScreenshotImageDataError(slideID, "empty image data")
	}
	imageBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, slidesScreenshotImageDataCauseError(slideID, err, "decode screenshot: %s", err)
	}
	var path string
	if outputPath != "" {
		path, err = writeUniqueScreenshotPath(runtime, slideScreenshotOutputPathForFormat(outputPath, ext), imageBytes)
	} else {
		fileBase := strings.TrimSpace(outputName)
		if fileBase == "" {
			fileBase = slideID
		}
		if fileBase == "" {
			fileBase = fallbackName
		}
		path, err = writeUniqueScreenshotFile(runtime, outputDir, fileBase, ext, imageBytes)
	}
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"slide_id":     slideID,
		"slide_number": slideScreenshotInt(item, "slide_number"),
		"format":       label,
		"path":         path,
		"size":         len(imageBytes),
	}, nil
}

func slideScreenshotOutputExtMatches(outputExt string, responseExt string) bool {
	outputExt = strings.ToLower(outputExt)
	if responseExt == "jpg" {
		return outputExt == ".jpg" || outputExt == ".jpeg"
	}
	return outputExt == "."+responseExt
}

func slideScreenshotOutputPathForFormat(outputPath string, responseExt string) string {
	outputExt := filepath.Ext(outputPath)
	if outputExt == "" {
		return outputPath + "." + responseExt
	}
	if slideScreenshotOutputExtMatches(outputExt, responseExt) {
		return outputPath
	}
	return strings.TrimSuffix(outputPath, outputExt) + "." + responseExt
}

func slideScreenshotListFileBase(presentationID string, item map[string]interface{}, index int) string {
	presentationID = strings.TrimSpace(presentationID)
	slideID := strings.TrimSpace(common.GetString(item, "slide_id"))
	slideNumber := slideScreenshotInt(item, "slide_number")
	if presentationID != "" {
		switch {
		case slideNumber > 0 && slideID != "":
			return fmt.Sprintf("%s_p%03d_%s", presentationID, slideNumber, slideID)
		case slideNumber > 0:
			return fmt.Sprintf("%s_p%03d", presentationID, slideNumber)
		case slideID != "":
			return fmt.Sprintf("%s_%s", presentationID, slideID)
		}
	}
	if slideID != "" {
		return slideID
	}
	if slideNumber := slideScreenshotInt(item, "slide_number"); slideNumber > 0 {
		return fmt.Sprintf("slide-%d", slideNumber)
	}
	return fmt.Sprintf("slide-%d", index+1)
}

func slideScreenshotFormat(item map[string]interface{}) (string, string, error) {
	format := slideScreenshotInt(item, "format")
	switch format {
	case 1:
		return "png", "png", nil
	case 2:
		return "jpg", "jpeg", nil
	default:
		return "", "", errs.NewAPIError(errs.SubtypeInvalidResponse, "unsupported screenshot format %d", format)
	}
}

func slidesScreenshotImageDataError(slideID string, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	if slideID != "" {
		msg = fmt.Sprintf("%s for slide %s", msg, slideID)
	}
	return errs.NewAPIError(errs.SubtypeInvalidResponse, "%s", msg)
}

func slidesScreenshotImageDataCauseError(slideID string, cause error, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	if slideID != "" {
		msg = fmt.Sprintf("%s for slide %s", msg, slideID)
	}
	return errs.NewAPIError(errs.SubtypeInvalidResponse, "%s", msg).WithCause(cause)
}

func slideScreenshotInt(item map[string]interface{}, key string) int {
	n, ok := util.ToFloat64(item[key])
	if !ok {
		return 0
	}
	return int(n)
}

func enrichSlidesScreenshotSelectorError(err error, slideNumbers []int) error {
	if len(slideNumbers) == 0 {
		return err
	}
	p, ok := errs.ProblemOf(err)
	if !ok {
		return err
	}
	if p.Hint == "" {
		p.Hint = "slide_numbers was rejected by the server; verify the page number exists in this presentation, or retry with --slide-id."
	}
	return err
}

func slidesScreenshotAPIDataError(data map[string]interface{}, format string, args ...interface{}) error {
	msg := fmt.Sprintf(format, args...)
	err := errs.NewAPIError(errs.SubtypeInvalidResponse, "%s; raw_data=%v", msg, summarizeScreenshotAPIData(data))
	if logID := strings.TrimSpace(common.GetString(data, "log_id")); logID != "" {
		err = err.WithLogID(logID)
	}
	return err
}

func isSlidesScreenshotPassthroughError(err error) bool {
	_, ok := errs.ProblemOf(err)
	return ok
}

func summarizeScreenshotAPIData(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[k] = summarizeScreenshotAPIData(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(x))
		for i, val := range x {
			if i >= 20 {
				out = append(out, fmt.Sprintf("<omitted %d more items>", len(x)-i))
				break
			}
			out = append(out, summarizeScreenshotAPIData(val))
		}
		return out
	case string:
		if len(x) > 512 {
			return fmt.Sprintf("<omitted string length=%d prefix=%q>", len(x), x[:64])
		}
		return x
	default:
		return x
	}
}

func safeScreenshotFileBase(base string) string {
	name := unsafeScreenshotFileCharRegex.ReplaceAllString(base, "_")
	name = strings.Trim(name, "._-")
	if name == "" {
		name = "slide"
	}
	return name
}

func writeUniqueScreenshotFile(runtime slidesScreenshotFileRuntime, outputDir string, fileBase string, ext string, imageBytes []byte) (string, error) {
	base := safeScreenshotFileBase(fileBase)
	return writeUniqueScreenshotPath(runtime, filepath.Join(outputDir, base+"."+ext), imageBytes)
}

func writeUniqueScreenshotPath(runtime slidesScreenshotFileRuntime, outputPath string, imageBytes []byte) (string, error) {
	ext := filepath.Ext(outputPath)
	base := strings.TrimSuffix(outputPath, ext)
	for i := 0; i < 1000; i++ {
		candidate := outputPath
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d%s", base, i+1, ext)
		}
		if _, err := runtime.FileIO().Stat(candidate); err == nil {
			continue
		} else if !isScreenshotFileNotExist(err) {
			return "", errs.NewInternalError(errs.SubtypeFileIO, "write screenshot %s: %v", candidate, err).WithCause(err)
		}
		if _, err := runtime.FileIO().Save(candidate, fileio.SaveOptions{}, bytes.NewReader(imageBytes)); err != nil {
			return "", common.WrapSaveErrorTyped(err)
		}
		resolvedPath, err := runtime.ResolveSavePath(candidate)
		if err != nil {
			return "", errs.NewInternalError(errs.SubtypeFileIO, "resolve saved screenshot path %s: %v", candidate, err).WithCause(err)
		}
		return resolvedPath, nil
	}
	return "", errs.NewInternalError(errs.SubtypeFileIO, "write screenshot %s: too many duplicate file names", outputPath)
}

func isScreenshotFileNotExist(err error) bool {
	return os.IsNotExist(err)
}
