// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/output"
)

const mailRulesReorderSchemaPath = "mail.user_mailbox.rules.reorder"

func isMailRulesReorder(opts *ServiceMethodOptions) bool {
	return opts != nil && opts.SchemaPath == mailRulesReorderSchemaPath
}

func completeMailRulesReorderRequest(ctx context.Context, ac *client.APIClient, opts *ServiceMethodOptions, request *client.RawApiRequest) error {
	if !isMailRulesReorder(opts) {
		return nil
	}

	requestedIDs, key, ok, err := mailRulesRequestedIDs(request.Data)
	if err != nil {
		return err
	}
	if !ok || len(requestedIDs) == 0 {
		return nil
	}
	if duplicates := duplicateStrings(requestedIDs); len(duplicates) > 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"duplicate mail rule id(s): %s", strings.Join(duplicates, ", ")).
			WithParam(key)
	}

	currentIDs, err := fetchCurrentMailRuleIDs(ctx, ac, *request)
	if err != nil {
		return err
	}
	completedIDs, unknown := completeMailRuleIDs(requestedIDs, currentIDs)
	if len(unknown) > 0 {
		return errs.NewValidationError(errs.SubtypeInvalidArgument,
			"unknown mail rule id(s): %s", strings.Join(unknown, ", ")).
			WithParam(key).
			WithHint("run: lark-cli mail user_mailbox.rules list --params '{\"user_mailbox_id\":\"<mailbox>\"}', then retry reorder with existing rule IDs")
	}

	body := request.Data.(map[string]interface{})
	body[key] = completedIDs
	return nil
}

func mailRulesRequestedIDs(data interface{}) ([]string, string, bool, error) {
	body, ok := data.(map[string]interface{})
	if !ok || body == nil {
		return nil, "", false, nil
	}
	for _, key := range []string{"rule_ids", "ruleIds"} {
		raw, ok := body[key]
		if !ok {
			continue
		}
		ids, err := stringList(raw)
		if err != nil {
			return nil, key, true, errs.NewValidationError(errs.SubtypeInvalidArgument,
				"%s must be an array of rule ID strings", key).
				WithParam(key).
				WithCause(err)
		}
		return ids, key, true, nil
	}
	return nil, "", false, nil
}

func stringList(raw interface{}) ([]string, error) {
	switch v := raw.(type) {
	case []string:
		return append([]string(nil), v...), nil
	case []interface{}:
		out := make([]string, 0, len(v))
		for i, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("item %d is %T", i, item)
			}
			out = append(out, strings.TrimSpace(s))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("value is %T", raw)
	}
}

func duplicateStrings(values []string) []string {
	seen := map[string]bool{}
	dups := map[string]bool{}
	var ordered []string
	for _, value := range values {
		if seen[value] {
			if !dups[value] {
				dups[value] = true
				ordered = append(ordered, value)
			}
			continue
		}
		seen[value] = true
	}
	return ordered
}

func fetchCurrentMailRuleIDs(ctx context.Context, ac *client.APIClient, request client.RawApiRequest) ([]string, error) {
	listURL, ok := strings.CutSuffix(request.URL, "/reorder")
	if !ok {
		return nil, errs.NewInternalError(errs.SubtypeUnknown,
			"mail rules reorder URL does not end with /reorder: %s", request.URL)
	}
	params := map[string]interface{}{}
	for k, v := range request.Params {
		params[k] = v
	}
	if _, ok := params["page_size"]; !ok {
		params["page_size"] = 100
	}
	result, err := ac.PaginateAll(ctx, client.RawApiRequest{
		Method: "GET",
		URL:    listURL,
		Params: params,
		As:     request.As,
	}, client.PaginationOptions{PageLimit: 0, PageDelay: -1, Identity: request.As})
	if err != nil {
		return nil, withMailRuleListHint(err)
	}
	if err := ac.CheckResponse(result, request.As); err != nil {
		return nil, withMailRuleListHint(err)
	}
	return mailRuleIDsFromListResult(result), nil
}

func mailRuleIDsFromListResult(result interface{}) []string {
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil
	}
	data, ok := resultMap["data"].(map[string]interface{})
	if !ok {
		return nil
	}
	arrayField := output.FindArrayField(data)
	if arrayField == "" {
		for _, key := range []string{"rules", "rule_list", "items"} {
			if _, ok := data[key]; ok {
				arrayField = key
				break
			}
		}
	}
	items, ok := data[arrayField].([]interface{})
	if !ok {
		return nil
	}
	ids := make([]string, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		for _, key := range []string{"rule_id", "id", "ruleId"} {
			if id, ok := m[key].(string); ok && id != "" {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids
}

func completeMailRuleIDs(requestedIDs, currentIDs []string) ([]string, []string) {
	current := map[string]bool{}
	for _, id := range currentIDs {
		current[id] = true
	}
	requested := map[string]bool{}
	var unknown []string
	for _, id := range requestedIDs {
		requested[id] = true
		if !current[id] {
			unknown = append(unknown, id)
		}
	}
	if len(unknown) > 0 {
		return nil, unknown
	}
	completed := append([]string(nil), requestedIDs...)
	for _, id := range currentIDs {
		if !requested[id] {
			completed = append(completed, id)
		}
	}
	return completed, nil
}

func withMailRuleListHint(err error) error {
	return withMailRuleHint(err, "could not fetch the complete mail rule list; reorder was not called")
}

func withMailRuleReorderHint(err error) error {
	return withMailRuleHint(err, "mail rules may have changed; run lark-cli mail user_mailbox.rules list again, then retry reorder")
}

func withMailRuleHint(err error, hint string) error {
	switch e := err.(type) {
	case *errs.ValidationError:
		return e.WithHint(hint)
	case *errs.AuthenticationError:
		return e.WithHint(hint)
	case *errs.PermissionError:
		return e.WithHint(hint)
	case *errs.ConfigError:
		return e.WithHint(hint)
	case *errs.NetworkError:
		return e.WithHint(hint)
	case *errs.APIError:
		return e.WithHint(hint)
	case *errs.SecurityPolicyError:
		return e.WithHint(hint)
	case *errs.ContentSafetyError:
		return e.WithHint(hint)
	case *errs.InternalError:
		return e.WithHint(hint)
	case *errs.ConfirmationRequiredError:
		return e.WithHint(hint)
	default:
		return err
	}
}

func mailRulesReorderCheckResponse(ac *client.APIClient, result interface{}, identity core.Identity) error {
	if err := ac.CheckResponse(result, identity); err != nil {
		return withMailRuleReorderHint(err)
	}
	return nil
}
