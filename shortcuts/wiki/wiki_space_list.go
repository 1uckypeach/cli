// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/shortcuts/common"
)

const (
	wikiSpacesAPIPath            = "/open-apis/wiki/v2/spaces"
	wikiSpaceListDefaultPageSize = 50
	wikiSpaceListMaxPageSize     = 50
)

type wikiSpaceListArgs struct {
	PageSize  int    `flag:"page-size" schema:"optional;default=50" doc:"page size, 1-50"`
	PageToken string `flag:"page-token" schema:"optional" doc:"page token; implies single-page fetch (no auto-pagination)"`
	PageAll   bool   `flag:"page-all" schema:"optional" doc:"automatically paginate through all pages (capped by --page-limit)"`
	PageLimit int    `flag:"page-limit" schema:"optional;default=10" doc:"max pages to fetch with --page-all (default 10, 0 = unlimited)"`
}

// Field order preserves the legacy map encoding order used by encoding/json.
type wikiSpace struct {
	Description string `json:"description" schema:"required" doc:"wiki space description"`
	Name        string `json:"name" schema:"required" doc:"wiki space name"`
	OpenSharing string `json:"open_sharing" schema:"required" doc:"open sharing setting"`
	SpaceID     string `json:"space_id" schema:"required" doc:"wiki space ID"`
	SpaceType   string `json:"space_type" schema:"required" doc:"wiki space type"`
	Visibility  string `json:"visibility" schema:"required" doc:"wiki space visibility"`
}

// Field order preserves the legacy map encoding order used by encoding/json.
type wikiSpaceListData struct {
	HasMore   bool        `json:"has_more" schema:"required" doc:"whether another page is available"`
	PageToken string      `json:"page_token" schema:"required" doc:"next page token when returned"`
	Spaces    []wikiSpace `json:"spaces" schema:"required;nonnullable" doc:"accessible wiki spaces"`
}

// WikiSpaceList lists all wiki spaces the caller has access to.
var WikiSpaceList = common.Define(common.Definition[wikiSpaceListArgs, wikiSpaceListData]{
	Metadata: common.CommandMetadata{
		Service: "wiki", Command: "+space-list", Description: "List wiki spaces accessible to the caller", Risk: common.RiskRead,
		Tips: []string{
			"Default fetches a single page (matches other list shortcuts in this CLI); pass --page-all to pull every page.",
			"The underlying API never returns the my_library personal library; resolve it via `wiki spaces get --params '{\"space_id\":\"my_library\"}'`.",
		},
		// Declare the narrowest valid scope: the upstream API accepts any of
		// wiki:wiki / wiki:wiki:readonly / wiki:space:retrieve, but preflight
		// performs exact-string matching. The narrow retrieve scope avoids false
		// rejection for otherwise valid tokens.
		Authorization: common.AuthorizationDefinition{Identities: map[common.Identity]common.IdentityAuthorization{
			common.IdentityUser: {RequiredScopes: []string{"wiki:space:retrieve"}},
			common.IdentityBot:  {RequiredScopes: []string{"wiki:space:retrieve"}},
		}},
	},
	Output: common.OutputDefinition{Meta: common.ResultMetaDefinition{Count: true}},
	Hooks: common.Hooks[wikiSpaceListArgs, wikiSpaceListData]{
		Validate: func(_ context.Context, _ common.CommandContext, args *wikiSpaceListArgs) error {
			return validateWikiSpaceListPagination(args)
		},
		DryRun: func(_ context.Context, _ common.CommandContext, args *wikiSpaceListArgs) *common.DryRunAPI {
			params := map[string]interface{}{"page_size": args.PageSize}
			if pageToken := strings.TrimSpace(args.PageToken); pageToken != "" {
				params["page_token"] = pageToken
			}
			dry := common.NewDryRunAPI()
			if wikiSpaceListShouldAutoPaginate(args) {
				dry.Desc("Auto-paginates through all pages (capped by --page-limit when > 0)")
			}
			return dry.GET(wikiSpacesAPIPath).Params(params)
		},
		Execute: func(ctx context.Context, command common.CommandContext, args *wikiSpaceListArgs) (common.Result[wikiSpaceListData], error) {
			warnIfConflictingWikiSpacePagingFlags(command.Stderr(), args)
			data, err := fetchWikiSpacesTyped(ctx, command, args)
			if err != nil {
				return common.Result[wikiSpaceListData]{}, err
			}
			fmt.Fprintf(command.Stderr(), "Found %d wiki space(s)\n", len(data.Spaces))
			return common.Success(data).WithMeta(common.CountMeta(len(data.Spaces))), nil
		},
		Renderers: map[string]common.Renderer[wikiSpaceListData]{"pretty": func(w io.Writer, data wikiSpaceListData) error {
			renderWikiSpacesPrettyTyped(w, data)
			return nil
		}},
	},
})

func validateWikiSpaceListPagination(args *wikiSpaceListArgs) error {
	if args.PageSize < 1 || args.PageSize > wikiSpaceListMaxPageSize {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--page-size must be between 1 and %d", wikiSpaceListMaxPageSize).WithParam("--page-size")
	}
	if args.PageLimit < 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--page-limit must be a non-negative integer").WithParam("--page-limit")
	}
	return nil
}

func wikiSpaceListShouldAutoPaginate(args *wikiSpaceListArgs) bool {
	return strings.TrimSpace(args.PageToken) == "" && args.PageAll
}

func warnIfConflictingWikiSpacePagingFlags(w io.Writer, args *wikiSpaceListArgs) {
	if strings.TrimSpace(args.PageToken) != "" && args.PageAll {
		fmt.Fprintln(w, "warning: --page-token is set, so --page-all is ignored (single-page fetch from the supplied cursor)")
	}
}

// fetchWikiSpacesTyped honours the four pagination flags:
//   - default (no --page-all, no --page-token): fetch a single page from the start
//   - --page-token X: fetch a single page starting at X (auto-pagination disabled)
//   - --page-all: pull subsequent pages, capped by --page-limit (default 10; 0 = unlimited)
//
// Spaces is always non-nil so JSON output stays as [] instead of null.
func fetchWikiSpacesTyped(ctx context.Context, command common.CommandContext, args *wikiSpaceListArgs) (wikiSpaceListData, error) {
	pageToken := strings.TrimSpace(args.PageToken)
	auto := wikiSpaceListShouldAutoPaginate(args)
	result := wikiSpaceListData{Spaces: make([]wikiSpace, 0)}
	for page := 0; ; page++ {
		params := map[string]interface{}{"page_size": args.PageSize}
		if pageToken != "" {
			params["page_token"] = pageToken
		}
		data, err := common.CallTypedAPI(ctx, command, "GET", wikiSpacesAPIPath, params, nil)
		if err != nil {
			return wikiSpaceListData{}, err
		}
		for _, item := range common.GetSlice(data, "items") {
			if record, ok := item.(map[string]interface{}); ok {
				result.Spaces = append(result.Spaces, parseWikiSpace(record))
			}
		}
		result.HasMore = common.GetBool(data, "has_more")
		result.PageToken = common.GetString(data, "page_token")
		if !auto || !result.HasMore || result.PageToken == "" || args.PageLimit > 0 && page+1 >= args.PageLimit {
			break
		}
		pageToken = result.PageToken
	}
	return result, nil
}

func parseWikiSpace(record map[string]interface{}) wikiSpace {
	return wikiSpace{
		Description: common.GetString(record, "description"),
		Name:        common.GetString(record, "name"),
		OpenSharing: common.GetString(record, "open_sharing"),
		SpaceID:     common.GetString(record, "space_id"),
		SpaceType:   common.GetString(record, "space_type"),
		Visibility:  common.GetString(record, "visibility"),
	}
}

func renderWikiSpacesPrettyTyped(w io.Writer, data wikiSpaceListData) {
	if len(data.Spaces) == 0 {
		if data.HasMore && data.PageToken != "" {
			fmt.Fprintln(w, "Current page is empty but the server reports more pages.")
			fmt.Fprintln(w, "Pass --page-all to walk every page, or --page-token to resume from the cursor below:")
			fmt.Fprintf(w, "  next page_token: %s\n", data.PageToken)
			return
		}
		fmt.Fprintln(w, "No wiki spaces found.")
		return
	}
	for index, space := range data.Spaces {
		fmt.Fprintf(w, "[%d] %s\n", index+1, valueOrDash(space.Name))
		fmt.Fprintf(w, "    space_id:     %s\n", valueOrDash(space.SpaceID))
		fmt.Fprintf(w, "    space_type:   %s\n", valueOrDash(space.SpaceType))
		fmt.Fprintf(w, "    visibility:   %s\n", valueOrDash(space.Visibility))
		fmt.Fprintf(w, "    open_sharing: %s\n", valueOrDash(space.OpenSharing))
		if space.Description != "" {
			fmt.Fprintf(w, "    description:  %s\n", space.Description)
		}
		fmt.Fprintln(w)
	}
	if data.HasMore && data.PageToken != "" {
		fmt.Fprintf(w, "Next page token: %s\n", data.PageToken)
	}
}

func valueOrDash(v interface{}) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return "-"
}

// validateWikiListPagination performs flag-level validation shared by Legacy
// wiki list shortcuts that still bind through RuntimeContext.
func validateWikiListPagination(runtime *common.RuntimeContext, maxPageSize int) error {
	if n := runtime.Int("page-size"); n < 1 || n > maxPageSize {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--page-size must be between 1 and %d", maxPageSize).WithParam("--page-size")
	}
	if n := runtime.Int("page-limit"); n < 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument, "--page-limit must be a non-negative integer").WithParam("--page-limit")
	}
	return nil
}

// wikiListShouldAutoPaginate reports whether a Legacy wiki list fetch loop
// should keep requesting pages.
func wikiListShouldAutoPaginate(runtime *common.RuntimeContext) bool {
	if strings.TrimSpace(runtime.Str("page-token")) != "" {
		return false
	}
	return runtime.Bool("page-all")
}

// warnIfConflictingPagingFlags preserves the Legacy warning used by +node-list.
func warnIfConflictingPagingFlags(runtime *common.RuntimeContext) {
	if strings.TrimSpace(runtime.Str("page-token")) != "" && runtime.Bool("page-all") {
		fmt.Fprintln(runtime.IO().ErrOut,
			"warning: --page-token is set, so --page-all is ignored (single-page fetch from the supplied cursor)")
	}
}
