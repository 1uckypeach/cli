// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package mail

import (
	"context"
	"strings"

	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/shortcuts/common"
)

var MailSenderAllowlist = newMailSenderListShortcut(senderListShortcutConfig{
	command:     "+sender-allowlist",
	resource:    "allow_senders",
	description: "List/search or update the user mailbox sender allowlist. Default mode lists entries; use --add or --remove to modify.",
})

var MailSenderBlocklist = newMailSenderListShortcut(senderListShortcutConfig{
	command:     "+sender-blocklist",
	resource:    "blocked_senders",
	description: "List/search or update the user mailbox sender blocklist. Default mode lists entries; use --add or --remove to modify.",
})

type senderListShortcutConfig struct {
	command     string
	resource    string
	description string
}

type senderListInput struct {
	Mode       string
	Addresses  []string
	SenderType int
}

func newMailSenderListShortcut(cfg senderListShortcutConfig) common.Shortcut {
	return common.Shortcut{
		Service:     "mail",
		Command:     cfg.command,
		Description: cfg.description,
		Risk:        "write",
		Scopes:      []string{"mail:user_mailbox:readonly"},
		ConditionalScopes: []string{
			"mail:user_mailbox",
		},
		AuthTypes: []string{"user"},
		HasFormat: true,
		Flags: []common.Flag{
			{Name: "mailbox", Default: "me", Desc: "Mailbox email address or user_mailbox_id (default: me)."},
			{Name: "query", Desc: "Prefix keyword to search sender email addresses or domains."},
			{Name: "page-size", Type: "int", Default: "20", Desc: "Page size for list/search mode."},
			{Name: "page-token", Desc: "Page token returned by a previous list/search response."},
			{Name: "add", Type: "string_array", Desc: "Sender email addresses or domains to add; comma-separated or repeat the flag."},
			{Name: "remove", Type: "string_array", Desc: "Sender email addresses or domains to remove; comma-separated or repeat the flag."},
			{Name: "type", Default: "email", Desc: "Sender type for --add: email or domain.", Enum: []string{"email", "domain"}},
			{Name: "yes", Type: "bool", Desc: "Confirm add/remove operation."},
		},
		Validate: func(ctx context.Context, rt *common.RuntimeContext) error {
			_, err := buildSenderListInput(rt)
			return err
		},
		DryRun: func(ctx context.Context, rt *common.RuntimeContext) *common.DryRunAPI {
			input, err := buildSenderListInput(rt)
			if err != nil {
				return common.NewDryRunAPI().Set("error", err.Error())
			}
			mailboxID := resolveMailboxID(rt)
			switch input.Mode {
			case "add":
				return common.NewDryRunAPI().
					Desc("Add senders to user mailbox sender list").
					POST(mailSenderListPath(mailboxID, cfg.resource, "batch_create")).
					Body(senderListAddBody(input.Addresses, input.SenderType))
			case "remove":
				return common.NewDryRunAPI().
					Desc("Remove senders from user mailbox sender list").
					POST(mailSenderListPath(mailboxID, cfg.resource, "batch_remove")).
					Body(map[string]interface{}{"senders": input.Addresses})
			default:
				return common.NewDryRunAPI().
					Desc("List or search user mailbox sender list").
					GET(mailSenderListPath(mailboxID, cfg.resource)).
					Params(mailSenderListParams(rt))
			}
		},
		Execute: func(ctx context.Context, rt *common.RuntimeContext) error {
			input, err := buildSenderListInput(rt)
			if err != nil {
				return err
			}
			mailboxID := resolveMailboxID(rt)
			switch input.Mode {
			case "add":
				if err := rt.EnsureScopes([]string{"mail:user_mailbox"}); err != nil {
					return err
				}
				if !rt.Bool("yes") {
					return cmdutil.RequireConfirmation("mail " + cfg.command)
				}
				data, err := rt.CallAPITyped("POST", mailSenderListPath(mailboxID, cfg.resource, "batch_create"), nil, senderListAddBody(input.Addresses, input.SenderType))
				if err != nil {
					return err
				}
				rt.Out(data, nil)
			case "remove":
				if err := rt.EnsureScopes([]string{"mail:user_mailbox"}); err != nil {
					return err
				}
				if !rt.Bool("yes") {
					return cmdutil.RequireConfirmation("mail " + cfg.command)
				}
				data, err := rt.CallAPITyped("POST", mailSenderListPath(mailboxID, cfg.resource, "batch_remove"), nil, map[string]interface{}{"senders": input.Addresses})
				if err != nil {
					return err
				}
				rt.Out(data, nil)
			default:
				data, err := rt.CallAPITyped("GET", mailSenderListPath(mailboxID, cfg.resource), mailSenderListParams(rt), nil)
				if err != nil {
					return err
				}
				rt.Out(data, nil)
			}
			return nil
		},
	}
}

func buildSenderListInput(rt *common.RuntimeContext) (senderListInput, error) {
	hasAdd := len(rt.StrArray("add")) > 0
	hasRemove := len(rt.StrArray("remove")) > 0
	if hasAdd && hasRemove {
		return senderListInput{}, mailValidationError("--add and --remove are mutually exclusive")
	}
	if !hasAdd && !hasRemove {
		if rt.Int("page-size") <= 0 {
			return senderListInput{}, mailValidationParamError("--page-size", "must be greater than 0")
		}
		if rt.Changed("type") {
			return senderListInput{}, mailValidationParamError("--type", "--type requires --add")
		}
		return senderListInput{Mode: "list"}, nil
	}
	if hasRemove {
		if rt.Changed("type") {
			return senderListInput{}, mailValidationParamError("--type", "--type requires --add")
		}
		addresses, err := normalizeMailSenderAddresses(rt.StrArray("remove"), "--remove")
		if err != nil {
			return senderListInput{}, err
		}
		return senderListInput{Mode: "remove", Addresses: addresses}, nil
	}
	addresses, err := normalizeMailSenderAddresses(rt.StrArray("add"), "--add")
	if err != nil {
		return senderListInput{}, err
	}
	senderType := 1
	if rt.Str("type") == "domain" {
		senderType = 2
	}
	return senderListInput{Mode: "add", Addresses: addresses, SenderType: senderType}, nil
}

func mailSenderListParams(rt *common.RuntimeContext) map[string]interface{} {
	params := map[string]interface{}{"page_size": rt.Int("page-size")}
	if query := strings.TrimSpace(rt.Str("query")); query != "" {
		params["keyword"] = query
	}
	if pageToken := strings.TrimSpace(rt.Str("page-token")); pageToken != "" {
		params["page_token"] = pageToken
	}
	return params
}

func senderListAddBody(addresses []string, senderType int) map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(addresses))
	for _, address := range addresses {
		items = append(items, map[string]interface{}{
			"sender":      address,
			"sender_type": senderType,
		})
	}
	return map[string]interface{}{"items": items}
}

func normalizeMailSenderAddresses(raw []string, param string) ([]string, error) {
	addresses := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, value := range raw {
		for _, part := range strings.Split(value, ",") {
			address := strings.TrimSpace(part)
			if address == "" {
				return nil, mailValidationParamError(param, "must not contain empty values")
			}
			if _, ok := seen[address]; ok {
				continue
			}
			seen[address] = struct{}{}
			addresses = append(addresses, address)
		}
	}
	if len(addresses) == 0 {
		return nil, mailValidationParamError(param, "must include at least one sender")
	}
	return addresses, nil
}

func mailSenderListPath(mailboxID, resource string, segments ...string) string {
	args := make([]string, 0, len(segments)+1)
	args = append(args, resource)
	args = append(args, segments...)
	return mailboxPath(mailboxID, args...)
}
