// Package selfupdate resolves the newest published release and replaces the
// running binary in place. Releases are published to GitHub Releases by
// GoReleaser (see .goreleaser.yaml); each release carries per-os/arch tarballs
// named abstr_{os}_{arch}.tar.gz alongside a sha256sums.txt manifest.
package selfupdate

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

// repoSlug is the GitHub owner/name that publishes releases.
const repoSlug = "abstraction-dev/cli"

// binaryName is the released binary/basename (matches .goreleaser.yaml).
const binaryName = "abstr"

// resolveTimeout is a backstop on the latest-version lookup so a stalled
// connection can't hang the command; callers also pass a context deadline.
const resolveTimeout = 10 * time.Second

// LatestVersion resolves the newest published tag (e.g. "v1.3.0") without hitting
// the GitHub API: a HEAD on releases/latest 302-redirects to the tagged release,
// and the tag is the final path segment of the Location header. This avoids the
// API's unauthenticated rate limit and returns no JSON to parse.
func LatestVersion(ctx context.Context) (string, error) {
	// Do not follow the redirect — we want to read its Location, not the page.
	client := &http.Client{
		Timeout: resolveTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	url := "https://github.com/" + repoSlug + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 300 || resp.StatusCode >= 400 {
		return "", fmt.Errorf("unexpected status resolving latest release: %s", resp.Status)
	}

	loc := resp.Header.Get("Location")
	if loc == "" {
		return "", fmt.Errorf("no Location header on releases/latest redirect")
	}

	tag := loc[strings.LastIndex(loc, "/")+1:]
	if !semver.IsValid(tag) {
		return "", fmt.Errorf("resolved latest tag %q is not valid semver", tag)
	}
	return tag, nil
}

// IsNewer reports whether latest is a strictly higher semver than current.
// A "dev" (or otherwise non-semver) current version is never considered
// upgradeable — local builds should not self-update.
func IsNewer(current, latest string) bool {
	cur := normalize(current)
	if !semver.IsValid(cur) || !semver.IsValid(latest) {
		return false
	}
	return semver.Compare(latest, cur) > 0
}

// normalize turns a bare or "abstr "-prefixed version into a semver-comparable
// string with a leading "v".
func normalize(v string) string {
	v = strings.TrimSpace(strings.TrimPrefix(v, binaryName))
	v = strings.TrimSpace(v)
	if v == "" {
		return v
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}
