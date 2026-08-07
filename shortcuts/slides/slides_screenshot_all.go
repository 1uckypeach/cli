// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package slides

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/errs"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

type slidesScreenshotAllDeck struct {
	SlideIDs   []string
	RevisionID int
}

const (
	maxSlidesScreenshotAllBatchRetries = 2
	maxSlidesScreenshotAllTotalRetries = 3
	slidesScreenshotAllBaseBackoff     = 500 * time.Millisecond
)

type slidesScreenshotAllRetryState struct {
	totalRetries int
	wait         func(context.Context, time.Duration) error
	jitter       func(time.Duration) time.Duration
}

func newSlidesScreenshotAllRetryState() *slidesScreenshotAllRetryState {
	return &slidesScreenshotAllRetryState{
		wait: func(ctx context.Context, delay time.Duration) error {
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-timer.C:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
		jitter: func(max time.Duration) time.Duration {
			if max <= 0 {
				return 0
			}
			return time.Duration(rand.Int63n(int64(max) + 1))
		},
	}
}

func slidesScreenshotAllCanRetry(err error) bool {
	if errs.IsRetryable(err) {
		return true
	}
	problem, ok := errs.ProblemOf(err)
	if !ok || problem.Category != errs.CategoryNetwork {
		return false
	}
	switch problem.Subtype {
	case errs.SubtypeNetworkTransport, errs.SubtypeNetworkTimeout, errs.SubtypeNetworkServer:
		return true
	default:
		return false
	}
}

func (state *slidesScreenshotAllRetryState) canRetryBatch(batchRetries int, err error) bool {
	return batchRetries < maxSlidesScreenshotAllBatchRetries &&
		state.totalRetries < maxSlidesScreenshotAllTotalRetries &&
		slidesScreenshotAllCanRetry(err)
}

func slidesScreenshotAllServerDelay(header http.Header) time.Duration {
	if header == nil {
		return 0
	}
	if value := strings.TrimSpace(header.Get("Retry-After")); value != "" {
		if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		if retryAt, err := http.ParseTime(value); err == nil {
			if delay := time.Until(retryAt); delay > 0 {
				return delay
			}
		}
	}
	if seconds, err := strconv.Atoi(strings.TrimSpace(header.Get("X-Ogw-Ratelimit-Reset"))); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}

func (state *slidesScreenshotAllRetryState) retryDelay(header http.Header, batchRetries int) time.Duration {
	if delay := slidesScreenshotAllServerDelay(header); delay > 0 {
		return delay
	}
	max := slidesScreenshotAllBaseBackoff << batchRetries
	return state.jitter(max)
}

func (state *slidesScreenshotAllRetryState) doRequest(ctx context.Context, runtime *common.RuntimeContext, method, url string, query larkcore.QueryParams, body interface{}) (map[string]interface{}, error) {
	requestRetries := 0
	for {
		req := &larkcore.ApiReq{
			HttpMethod:  method,
			ApiPath:     url,
			QueryParams: query,
			Body:        body,
		}
		resp, err := runtime.DoAPIWithContext(ctx, req)
		var header http.Header
		var data map[string]interface{}
		if err == nil {
			header = resp.Header
			data, err = runtime.ClassifyAPIResponse(resp)
		}
		if err == nil {
			if data == nil {
				data = map[string]interface{}{}
			}
			if logID := strings.TrimSpace(resp.Header.Get("x-tt-logid")); logID != "" {
				data["log_id"] = logID
			}
			return data, nil
		}
		err = errs.WrapInternal(err)
		if !state.canRetryBatch(requestRetries, err) {
			return data, err
		}
		delay := state.retryDelay(header, requestRetries)
		requestRetries++
		state.totalRetries++
		if waitErr := state.wait(ctx, delay); waitErr != nil {
			return data, errs.NewNetworkError(errs.SubtypeNetworkTransport,
				"slides screenshot --all retry wait was canceled: %v", waitErr).WithCause(waitErr)
		}
	}
}

func (state *slidesScreenshotAllRetryState) doBatch(ctx context.Context, runtime *common.RuntimeContext, url string, slideIDs []string) (map[string]interface{}, error) {
	return state.doRequest(ctx, runtime, "POST", url, nil, map[string]interface{}{"slide_ids": slideIDs})
}

func extractSlidesScreenshotAllSlideIDs(content string) ([]string, error) {
	decoder := xml.NewDecoder(strings.NewReader(content))
	depth := 0
	rootDepth := 0
	rootSeen := false
	ids := make([]string, 0, 16)
	seen := map[string]struct{}{}

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, errs.NewInternalError(errs.SubtypeInvalidResponse, "parse presentation XML: %v", err).WithCause(err)
		}
		switch node := token.(type) {
		case xml.StartElement:
			depth++
			if !rootSeen {
				rootSeen = true
				rootDepth = depth
				if node.Name.Local != "presentation" {
					return nil, errs.NewInternalError(errs.SubtypeInvalidResponse, "presentation XML root is <%s>, want <presentation>", node.Name.Local)
				}
				continue
			}
			if depth != rootDepth+1 || node.Name.Local != "slide" {
				continue
			}
			id := ""
			for _, attr := range node.Attr {
				if attr.Name.Local == "id" {
					id = strings.TrimSpace(attr.Value)
					break
				}
			}
			if id == "" {
				return nil, errs.NewInternalError(errs.SubtypeInvalidResponse, "presentation XML contains a slide with empty id")
			}
			if _, ok := seen[id]; ok {
				return nil, errs.NewInternalError(errs.SubtypeInvalidResponse, "presentation XML contains duplicate slide id %q", id)
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		case xml.EndElement:
			depth--
		}
	}
	if !rootSeen {
		return nil, errs.NewInternalError(errs.SubtypeInvalidResponse, "presentation XML is empty")
	}
	if len(ids) == 0 {
		return nil, errs.NewValidationError(errs.SubtypeFailedPrecondition, "presentation contains no slides").
			WithHint("add at least one slide before requesting --all screenshots")
	}
	return ids, nil
}

func slidesScreenshotAllBatches(slideIDs []string) ([][]string, error) {
	if len(slideIDs) == 0 {
		return nil, errs.NewValidationError(errs.SubtypeFailedPrecondition, "presentation contains no slides").
			WithHint("add at least one slide before requesting --all screenshots")
	}
	if len(slideIDs) > maxSlidesPerAllScreenshot {
		return nil, errs.NewValidationError(errs.SubtypeFailedPrecondition,
			"presentation has %d slides, exceeding --all maximum of %d", len(slideIDs), maxSlidesPerAllScreenshot).
			WithHint("screenshot an explicit subset with --slide-id, or split the deck into smaller presentations")
	}
	batches := make([][]string, 0, (len(slideIDs)+maxSlidesPerScreenshot-1)/maxSlidesPerScreenshot)
	for start := 0; start < len(slideIDs); start += maxSlidesPerScreenshot {
		end := start + maxSlidesPerScreenshot
		if end > len(slideIDs) {
			end = len(slideIDs)
		}
		batches = append(batches, append([]string(nil), slideIDs[start:end]...))
	}
	return batches, nil
}

func fetchSlidesScreenshotAllDeck(ctx context.Context, runtime *common.RuntimeContext, retry *slidesScreenshotAllRetryState, presentationID string) (slidesScreenshotAllDeck, error) {
	data, err := retry.doRequest(
		ctx,
		runtime,
		"GET",
		fmt.Sprintf("/open-apis/slides_ai/v1/xml_presentations/%s", validate.EncodePathSegment(presentationID)),
		larkcore.QueryParams{"revision_id": []string{"-1"}},
		nil,
	)
	if err != nil {
		return slidesScreenshotAllDeck{}, err
	}
	presentation := common.GetMap(data, "xml_presentation")
	content := common.GetString(presentation, "content")
	if content == "" {
		return slidesScreenshotAllDeck{}, errs.NewInternalError(errs.SubtypeInvalidResponse, "slides screenshot --all returned empty xml_presentation.content")
	}
	ids, err := extractSlidesScreenshotAllSlideIDs(content)
	if err != nil {
		return slidesScreenshotAllDeck{}, err
	}
	deck := slidesScreenshotAllDeck{SlideIDs: ids}
	if revisionID := common.GetFloat(presentation, "revision_id"); revisionID > 0 {
		deck.RevisionID = int(revisionID)
	}
	return deck, nil
}

func orderedSlidesScreenshotBatchData(data map[string]interface{}, expectedIDs []string) (map[string]interface{}, error) {
	items := common.GetSlice(data, "slide_images")
	byID := make(map[string]map[string]interface{}, len(items))
	for i, raw := range items {
		item, ok := raw.(map[string]interface{})
		if !ok {
			return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned invalid slide_images[%d]", i)
		}
		id := strings.TrimSpace(common.GetString(item, "slide_id"))
		if id == "" {
			return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned slide_images[%d] without slide_id", i)
		}
		if _, duplicate := byID[id]; duplicate {
			return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned duplicate slide_id %q", id)
		}
		byID[id] = item
	}

	expected := make(map[string]struct{}, len(expectedIDs))
	ordered := make([]interface{}, 0, len(expectedIDs))
	for _, id := range expectedIDs {
		expected[id] = struct{}{}
		item, ok := byID[id]
		if !ok {
			return nil, slidesScreenshotAPIDataError(data, "slides screenshot omitted requested slide_id %q", id)
		}
		ordered = append(ordered, item)
	}
	for id := range byID {
		if _, ok := expected[id]; !ok {
			return nil, slidesScreenshotAPIDataError(data, "slides screenshot returned unrequested slide_id %q", id)
		}
	}
	orderedData := make(map[string]interface{}, len(data)+1)
	for key, value := range data {
		orderedData[key] = value
	}
	orderedData["slide_images"] = ordered
	return orderedData, nil
}

func saveSlidesScreenshotAllBatch(runtime *common.RuntimeContext, data map[string]interface{}, expectedIDs []string, outputDir, presentationID string, offset int) ([]map[string]interface{}, error) {
	ordered, err := orderedSlidesScreenshotBatchData(data, expectedIDs)
	if err != nil {
		return nil, err
	}
	items := common.GetSlice(ordered, "slide_images")
	saved := make([]map[string]interface{}, 0, len(items))
	for i, raw := range items {
		item := raw.(map[string]interface{}) // orderedSlidesScreenshotBatchData validated this shape.
		result, err := saveSlideScreenshotImage(runtime, item, outputDir,
			slideScreenshotListFileBase(presentationID, item, offset+i), "", "")
		if err != nil {
			if isSlidesScreenshotPassthroughError(err) {
				return saved, err
			}
			return saved, slidesScreenshotAPIDataError(data, "slides screenshot returned invalid slide image: %v", err)
		}
		saved = append(saved, result)
	}
	return saved, nil
}

func slidesScreenshotAllErrorData(err error) map[string]interface{} {
	out := map[string]interface{}{"message": err.Error(), "retryable": slidesScreenshotAllCanRetry(err)}
	if problem, ok := errs.ProblemOf(err); ok {
		out["type"] = problem.Category
		out["subtype"] = problem.Subtype
		if problem.Code != 0 {
			out["code"] = problem.Code
		}
		if problem.Hint != "" {
			out["hint"] = problem.Hint
		}
		if problem.LogID != "" {
			out["log_id"] = problem.LogID
		}
	}
	return out
}

func slidesScreenshotAllPartialFailure(runtime *common.RuntimeContext, presentationID string, deck slidesScreenshotAllDeck, batches [][]string, batchIndex, batchesCompleted, retriesUsed int, saved []map[string]interface{}, cause error, outputTarget slidesScreenshotOutputTarget) error {
	result := map[string]interface{}{
		"xml_presentation_id": presentationID,
		"mode":                "all",
		"status":              "partial_failure",
		"screenshots":         saved,
		"failed_batches": []map[string]interface{}{{
			"batch":     batchIndex + 1,
			"slide_ids": batches[batchIndex],
			"error":     slidesScreenshotAllErrorData(cause),
		}},
		"summary": map[string]interface{}{
			"total":             len(deck.SlideIDs),
			"succeeded":         len(saved),
			"failed":            len(deck.SlideIDs) - len(saved),
			"batches_total":     len(batches),
			"batches_completed": batchesCompleted,
			"retries_used":      retriesUsed,
		},
	}
	if deck.RevisionID > 0 {
		result["source_revision_id"] = deck.RevisionID
	}
	setSlidesScreenshotResultOutput(result, outputTarget, saved)
	return runtime.OutPartialFailure(result, nil)
}

func executeAllSlidesScreenshot(ctx context.Context, runtime *common.RuntimeContext, presentationID string, outputTarget slidesScreenshotOutputTarget) error {
	retry := newSlidesScreenshotAllRetryState()
	deck, err := fetchSlidesScreenshotAllDeck(ctx, runtime, retry, presentationID)
	if err != nil {
		return err
	}
	batches, err := slidesScreenshotAllBatches(deck.SlideIDs)
	if err != nil {
		return err
	}

	url := fmt.Sprintf(
		"/open-apis/slides_ai/v1/xml_presentations/%s/slide_images",
		validate.EncodePathSegment(presentationID),
	)
	saved := make([]map[string]interface{}, 0, len(deck.SlideIDs))
	batchesCompleted := 0
	for batchIndex, batch := range batches {
		if err := ctx.Err(); err != nil {
			cancelErr := errs.NewNetworkError(errs.SubtypeNetworkTransport, "slides screenshot --all was canceled: %v", err).WithCause(err)
			if len(saved) > 0 {
				return slidesScreenshotAllPartialFailure(runtime, presentationID, deck, batches, batchIndex, batchesCompleted, retry.totalRetries, saved, cancelErr, outputTarget)
			}
			return cancelErr
		}
		data, err := retry.doBatch(ctx, runtime, url, batch)
		if err != nil {
			if len(saved) > 0 {
				return slidesScreenshotAllPartialFailure(runtime, presentationID, deck, batches, batchIndex, batchesCompleted, retry.totalRetries, saved, err, outputTarget)
			}
			return err
		}
		batchSaved, err := saveSlidesScreenshotAllBatch(runtime, data, batch, outputTarget.safeOutputDir, presentationID, len(saved))
		saved = append(saved, batchSaved...)
		if err != nil {
			if len(saved) > 0 {
				return slidesScreenshotAllPartialFailure(runtime, presentationID, deck, batches, batchIndex, batchesCompleted, retry.totalRetries, saved, err, outputTarget)
			}
			return err
		}
		batchesCompleted++
	}

	result := map[string]interface{}{
		"xml_presentation_id": presentationID,
		"mode":                "all",
		"screenshots":         saved,
		"summary": map[string]interface{}{
			"total":             len(deck.SlideIDs),
			"succeeded":         len(saved),
			"failed":            0,
			"batches_total":     len(batches),
			"batches_completed": len(batches),
			"retries_used":      retry.totalRetries,
		},
	}
	if deck.RevisionID > 0 {
		result["source_revision_id"] = deck.RevisionID
	}
	setSlidesScreenshotResultOutput(result, outputTarget, saved)
	runtime.Out(result, nil)
	return nil
}
