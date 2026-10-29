package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
)

// ReportTypeBranch is a PRReview's ReportType for a pre-review of a branch
// that has no pull request yet.
const ReportTypeBranch = "BRANCH"

// StartReviewRequest names a review from a checkout: a pull request by its
// URL, or a branch by its repository (owner/name).
type StartReviewRequest struct {
	PRURL      string `json:"prUrl,omitempty"`
	Repository string `json:"repository,omitempty"`
	Branch     string `json:"branch,omitempty"`
	BaseBranch string `json:"baseBranch,omitempty"`
}

// StartedReview is the review a start launched. Deferred means it waits for
// its head to be indexed and has no report to follow yet.
type StartedReview struct {
	ReportID string `json:"reportId"`
	Deferred bool   `json:"deferred"`
}

// ReviewProgress is one snapshot of a running review: its phases in order
// and the headline of the one under way.
type ReviewProgress struct {
	// State is running until the review finishes, then completed, failed or
	// cancelled.
	State    string       `json:"state"`
	Headline string       `json:"headline"`
	Fraction float64      `json:"fraction"`
	Steps    []ReviewStep `json:"steps"`
	// Raw is the snapshot as the server sent it.
	Raw json.RawMessage `json:"-"`
}

// ReviewStep is one phase of a review (queued, packages, builds, review or
// checks), or one of the review job's own steps. Done and Total count its
// work when it has a count.
type ReviewStep struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Detail string `json:"detail"`
	// State is pending, running, completed, skipped or failed.
	State    string       `json:"state"`
	Done     *int64       `json:"done,string"`
	Total    *int64       `json:"total,string"`
	Substeps []ReviewStep `json:"substeps"`
}

// ReviewFinished is how a followed review ended.
type ReviewFinished struct {
	Status       string `json:"status"`
	Verdict      string `json:"verdict"`
	OpenFindings int    `json:"openFindings"`
	URL          string `json:"url"`
	// Raw is the event as the server sent it.
	Raw json.RawMessage `json:"-"`
}

// ReviewIssue is one finding of a review.
type ReviewIssue struct {
	Severity    string `json:"severity"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Function    string `json:"function"`
	Description string `json:"description"`
	FilePath    string `json:"filePath"`
	StartLine   int    `json:"startLine"`
	// State is ACTIVE while the latest run still reports the finding.
	State     string `json:"state"`
	PreReview bool   `json:"preReview"`
	HiddenBy  string `json:"hiddenByInsightId"`
}

// Open reports whether the finding is one the review currently raises: still
// reported, not hidden, and not left over from before its PR opened.
func (i ReviewIssue) Open() bool {
	return i.State == "ACTIVE" && !i.PreReview && i.HiddenBy == ""
}

type getReviewResponse struct {
	Issues []ReviewIssue `json:"issues"`
}

// StartReview starts a review in the workspace.
func (c *Client) StartReview(ctx context.Context, workspace string, start StartReviewRequest) (StartedReview, error) {
	body, err := json.Marshal(start)
	if err != nil {
		return StartedReview{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.workspaceURL(workspace, "/reviews"), bytes.NewReader(body))
	if err != nil {
		return StartedReview{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	var out StartedReview
	return out, c.do(req, &out)
}

// BranchReviews lists the caller's open pre-reviews in the workspace. A
// workspace without pre-reviews has none.
func (c *Client) BranchReviews(ctx context.Context, workspace string) ([]PRReview, error) {
	var out listPRReviewsResponse
	err := c.getJSON(ctx, c.workspaceURL(workspace, "/branch-reviews"), &out)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil, nil
	}
	return out.Reviews, err
}

// ReviewIssues lists a review's findings, in rank order.
func (c *Client) ReviewIssues(ctx context.Context, workspace, reportID string) ([]ReviewIssue, error) {
	var out getReviewResponse
	err := c.getJSON(ctx, c.workspaceURL(workspace, "/pr-reviews/"+url.PathEscape(reportID)), &out)
	return out.Issues, err
}

// FollowReview delivers each snapshot of the review's progress to onProgress
// until it finishes, and returns how it ended.
func (c *Client) FollowReview(ctx context.Context, workspace, reportID string, onProgress func(ReviewProgress)) (ReviewFinished, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.workspaceURL(workspace, "/pr-reviews/"+url.PathEscape(reportID)+"/progress"), nil)
	if err != nil {
		return ReviewFinished{}, err
	}
	req.Header.Set("Accept", "text/event-stream")
	c.auth(req)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return ReviewFinished{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ReviewFinished{}, errorFrom(resp)
	}

	r := newSSEReader(resp.Body)
	for {
		ev, err := r.next()
		if errors.Is(err, io.EOF) {
			return ReviewFinished{}, errors.New("the progress stream ended before the review finished")
		}
		if err != nil {
			return ReviewFinished{}, err
		}
		switch ev.Type {
		case "progress":
			progress := ReviewProgress{Raw: json.RawMessage(ev.Data)}
			if json.Unmarshal(progress.Raw, &progress) == nil {
				onProgress(progress)
			}
		case "finished":
			finished := ReviewFinished{Raw: json.RawMessage(ev.Data)}
			return finished, json.Unmarshal(finished.Raw, &finished)
		}
	}
}
