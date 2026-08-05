// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package event

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/client"
	"github.com/larksuite/cli/internal/core"
)

// consumeRuntime routes event.APIClient calls through the shared client.APIClient with a pinned identity.
type consumeRuntime struct {
	client         *client.APIClient
	accessIdentity core.Identity
}

func (r *consumeRuntime) CallAPI(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	resp, err := r.client.DoAPI(ctx, client.RawApiRequest{
		Method: method,
		URL:    path,
		Data:   body,
		As:     r.accessIdentity,
	})
	if err != nil {
		if _, ok := errs.ProblemOf(err); ok {
			return nil, withEventAPIContext(err, method, path)
		}
		return nil, errs.NewNetworkError(errs.SubtypeNetworkTransport,
			"api %s %s: %s", method, path, err).WithCause(err)
	}
	// Event's non-JSON gateway contract includes method/path context and a
	// tighter body bound. Preserve that domain-specific fallback, while routing
	// HTTP 429 through the shared API classifier first.
	ct := resp.Header.Get("Content-Type")
	if resp.StatusCode >= http.StatusBadRequest && !client.IsJSONContentType(ct) && ct != "" {
		if resp.StatusCode == http.StatusTooManyRequests {
			_, classified := client.ClassifyAPIResponse(resp, nil)
			return nil, classified
		}
		const maxBodyEcho = 256
		body := string(resp.RawBody)
		if len(body) > maxBodyEcho {
			body = body[:maxBodyEcho] + "…(truncated)"
		}
		if resp.StatusCode >= http.StatusInternalServerError {
			return nil, errs.NewNetworkError(errs.SubtypeNetworkServer,
				"api %s %s returned %d: %s", method, path, resp.StatusCode, body).WithRetryable()
		}
		return nil, errs.NewInternalError(errs.SubtypeInvalidResponse,
			"api %s %s returned %d: %s", method, path, resp.StatusCode, body)
	}
	_, classified := client.ClassifyAPIResponse(resp, func(result interface{}) error {
		return r.client.CheckResponse(result, r.accessIdentity)
	})
	if classified != nil {
		return json.RawMessage(resp.RawBody), withEventAPIContext(classified, method, path)
	}
	return json.RawMessage(resp.RawBody), nil
}

func withEventAPIContext(err error, method, path string) error {
	if problem, ok := errs.ProblemOf(err); ok {
		if problem.Category == errs.CategoryInternal && problem.Subtype == errs.SubtypeInvalidResponse {
			problem.Message = fmt.Sprintf("api %s %s: %s", method, path, problem.Message)
		}
		if problem.Category == errs.CategoryNetwork && problem.Subtype == errs.SubtypeNetworkServer {
			problem.Retryable = true
		}
	}
	return err
}
