package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
)

func TestStartReview(t *testing.T) {
	var gotPath string
	var gotBody map[string]any
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte(`{"reportId":"41"}`))
	})

	started, err := client.StartReview(context.Background(), "ws", StartReviewRequest{Repository: "acme/api", Branch: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/api/workspaces/ws/reviews" || gotBody["repository"] != "acme/api" || gotBody["branch"] != "feature" || gotBody["prUrl"] != nil {
		t.Fatalf("request %s %v", gotPath, gotBody)
	}
	if started.ReportID != "41" || started.Deferred {
		t.Fatalf("started %+v", started)
	}
}

// Progress snapshots arrive in order, protojson's stringified int64 counts
// decode, and the finished event ends the follow.
func TestFollowReview(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/workspaces/ws/pr-reviews/41/progress" {
			t.Errorf("path %s", r.URL.Path)
		}
		fmt.Fprint(w, ": ping\n\n")
		fmt.Fprint(w, "event: progress\ndata: {\"state\":\"running\",\"headline\":\"Queued · waiting for a worker\"}\n\n")
		fmt.Fprint(w, "event: progress\ndata: {\"state\":\"running\",\"headline\":\"Reviewing\",\"fraction\":0.5,\"steps\":[{\"key\":\"review\",\"name\":\"Review\",\"state\":\"running\",\"done\":\"34\",\"total\":\"77\"}]}\n\n")
		fmt.Fprint(w, "event: finished\ndata: {\"status\":\"COMPLETED\",\"verdict\":\"FAIL\",\"openFindings\":2,\"url\":\"https://app/r/41\"}\n\n")
	})

	var headlines []string
	var review ReviewStep
	finished, err := client.FollowReview(context.Background(), "ws", "41", func(progress ReviewProgress) {
		headlines = append(headlines, progress.Headline)
		if len(progress.Steps) > 0 {
			review = progress.Steps[0]
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(headlines) != 2 || headlines[1] != "Reviewing" {
		t.Fatalf("headlines %q", headlines)
	}
	if review.Done == nil || *review.Done != 34 || *review.Total != 77 {
		t.Fatalf("review step %+v", review)
	}
	if finished.Status != "COMPLETED" || finished.Verdict != "FAIL" || finished.OpenFindings != 2 || string(finished.Raw) == "" {
		t.Fatalf("finished %+v", finished)
	}
}

func TestFollowReviewEndingEarlyIsAnError(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "event: progress\ndata: {\"state\":\"running\"}\n\n")
	})
	if _, err := client.FollowReview(context.Background(), "ws", "41", func(ReviewProgress) {}); err == nil {
		t.Fatal("a stream that ends before finished must fail")
	}
}

// A workspace without pre-reviews 404s its branch list, which reads as empty.
func TestBranchReviewsWithoutPreReviews(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	reviews, err := client.BranchReviews(context.Background(), "ws")
	if err != nil || len(reviews) != 0 {
		t.Fatalf("got %v, %v", reviews, err)
	}
}

func TestReviewIssueOpen(t *testing.T) {
	for _, tc := range []struct {
		issue ReviewIssue
		open  bool
	}{
		{ReviewIssue{State: "ACTIVE"}, true},
		{ReviewIssue{State: "RESOLVED"}, false},
		{ReviewIssue{State: "ACTIVE", PreReview: true}, false},
		{ReviewIssue{State: "ACTIVE", HiddenBy: "i1"}, false},
	} {
		if tc.issue.Open() != tc.open {
			t.Errorf("%+v open = %v", tc.issue, !tc.open)
		}
	}
}
