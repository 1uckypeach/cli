// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package config

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/build"
	"github.com/larksuite/cli/internal/cmdutil"
	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/credential"
)

// probeTimeout is the total wall-clock budget for the credential probe step
// (covering both TAT acquisition and the subsequent probe request).
const probeTimeout = 3 * time.Second

// runProbe runs a best-effort credential validation after config init has
// persisted the App ID and App Secret. It returns a non-nil error only for a
// deterministic credential-rejection signal; every other outcome returns nil
// so that valid configurations and transient/upstream noise never block the
// command.
//
// The function performs up to two HTTP calls in series, bounded by
// probeTimeout:
//
//  1. A TAT request using the just-saved credentials. credential.FetchTAT
//     returns a typed errs.* error (via the shared classifyTATResponseCode)
//     only when the unified Token Endpoint deterministically rejected the
//     credentials — an OAuth2 invalid_client / unauthorized_client classified as
//     CategoryConfig / SubtypeInvalidClient, or whatever codemeta maps. That
//     typed error is propagated so the root dispatcher renders the canonical
//     envelope and `config init` exits non-zero — identical to how every other
//     token-resolving command reports the same bad credentials. A retryable
//     api/rate_limit error and ambiguous untyped failures (transport errors,
//     transient 5xx/server_error, JSON parse errors, timeouts) are swallowed so
//     valid configurations are never disturbed by upstream noise.
//
//  2. If TAT succeeded, a POST to the probe endpoint is fired. The outcome of
//     that call (success, server error, timeout, parse failure) is always
//     ignored — return nil regardless.
func runProbe(parent context.Context, factory *cmdutil.Factory, appID, appSecret string, brand core.LarkBrand) error {
	if factory == nil {
		return nil
	}
	httpClient, err := factory.HttpClient()
	if err != nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(parent, probeTimeout)
	defer cancel()

	token, err := credential.FetchTAT(ctx, httpClient, brand, appID, appSecret)
	if err != nil {
		// A retryable API rate limit is expected probe noise. Other typed
		// errors are deterministic and must remain visible to the caller.
		if ignoreProbeTATError(err) {
			return nil
		}
		if _, ok := errs.ProblemOf(err); ok {
			return err
		}
		return nil
	}

	// TAT succeeded — fire the probe call. Any outcome is ignored.
	url := core.ResolveEndpoints(brand).Open + "/open-apis/application/v6/larksuite_cli_app/probe"
	body := []byte(fmt.Sprintf(`{"from":"lark-cli/%s"}`, build.Version))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func ignoreProbeTATError(err error) bool {
	problem, ok := errs.ProblemOf(err)
	return ok && problem.Category == errs.CategoryAPI && problem.Subtype == errs.SubtypeRateLimit && problem.Retryable
}
