package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/abstraction-dev/cli/internal/config"
	"github.com/abstraction-dev/cli/internal/render"
	"github.com/abstraction-dev/cli/internal/selfupdate"
)

// updateCheckInterval throttles the background "is there a newer release?"
// check so we hit the network at most once per window.
const updateCheckInterval = 24 * time.Hour

// updateCheckTimeout bounds the background check so it never lingers.
const updateCheckTimeout = 3 * time.Second

// updateCheck is a background release check started alongside the main task. Its
// result is consumed once, at the end of the run, by finish.
type updateCheck struct {
	cfg    *config.Config
	result chan string // resolved latest tag, or "" if not checked/failed
}

// startUpdateCheck kicks off a throttled, concurrent release check. It returns a
// no-op check when updates are disabled (dev build, CI, opt-out, non-TTY) or
// when the last check was recent — in which case the run still acts on the
// cached LatestSeen. The check runs concurrently with the user's task so its
// latency is hidden behind work that is already happening.
func startUpdateCheck(ctx context.Context, cfg *config.Config) *updateCheck {
	uc := &updateCheck{cfg: cfg, result: make(chan string, 1)}

	if !updatesEnabled() || !checkDue(cfg) {
		uc.result <- ""
		return uc
	}

	go func() {
		cctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
		defer cancel()
		tag, err := selfupdate.LatestVersion(cctx)
		if err != nil {
			uc.result <- "" // fail silent; background checks never nag on error
			return
		}
		uc.result <- tag
	}()
	return uc
}

// finish consumes the check result, persists the cache, and either prints an
// upgrade notice or (when auto-upgrade is enabled) applies the update in place.
// It is safe to call once per run, after the main task completes.
func finish(ctx context.Context, uc *updateCheck) {
	latest := <-uc.result

	// Persist the cache when we actually resolved something new this run. Reload
	// from disk first so we don't clobber fields the task wrote (e.g. workspace).
	if latest != "" {
		if fresh, err := config.Load(uc.cfg.FilePath()); err == nil {
			fresh.LastUpdateCheck = nowUTC()
			fresh.LatestSeen = latest
			_ = fresh.Save()
		}
	}

	// Decide against the newest tag we know about: this run's result if we have
	// one, else what the previous check cached.
	newest := latest
	if newest == "" {
		newest = uc.cfg.LatestSeen
	}
	if newest == "" || !selfupdate.IsNewer(version, newest) {
		return
	}

	r := newRenderer()
	if uc.cfg.AutoUpgrade {
		applyAuto(ctx, r, newest)
		return
	}
	r.Info(fmt.Sprintf("A new abstr (%s) is available — run 'abstr upgrade' to update.", newest))
}

// applyAuto performs an opt-in automatic upgrade, degrading to a notice when the
// binary lives somewhere we must not overwrite (a package-manager install).
func applyAuto(ctx context.Context, r *render.Renderer, tag string) {
	r.Status("Upgrading abstr to " + tag + "…")
	if err := selfupdate.Apply(ctx, tag); err != nil {
		if errors.Is(err, selfupdate.ErrUnmanagedInstall) {
			r.Info(fmt.Sprintf("A new abstr (%s) is available, but this install is managed elsewhere — upgrade with your package manager.", tag))
			return
		}
		r.Warn("auto-upgrade failed: " + err.Error())
		return
	}
	r.Info(fmt.Sprintf("Upgraded abstr to %s — it takes effect on your next run.", tag))
}

// updatesEnabled reports whether background update behavior should run at all.
// Local dev builds, CI, an explicit opt-out, and non-interactive output are all
// exempt so scripts and pipelines stay quiet and deterministic.
func updatesEnabled() bool {
	if version == "dev" {
		return false
	}
	if os.Getenv("ABSTR_NO_UPDATE_CHECK") != "" {
		return false
	}
	if os.Getenv("CI") != "" {
		return false
	}
	return render.IsTerminal(os.Stderr)
}

// checkDue reports whether enough time has passed since the last check.
func checkDue(cfg *config.Config) bool {
	if cfg.LastUpdateCheck == "" {
		return true
	}
	last, err := time.Parse(time.RFC3339, cfg.LastUpdateCheck)
	if err != nil {
		return true
	}
	return time.Since(last) >= updateCheckInterval
}

func nowUTC() string { return time.Now().UTC().Format(time.RFC3339) }

// runUpgrade implements `abstr upgrade`: resolve the latest release and replace
// the running binary. With --check it only reports availability.
func runUpgrade(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("abstr upgrade", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var checkOnly bool
	fs.BoolVar(&checkOnly, "check", false, "report whether an update is available without installing")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	r := newRenderer()

	latest, err := selfupdate.LatestVersion(ctx)
	if err != nil {
		r.Error("abstr: could not resolve the latest release: " + err.Error())
		return exitRuntime
	}

	isDev := version == "dev"
	if !isDev && !selfupdate.IsNewer(version, latest) {
		r.Info(fmt.Sprintf("abstr is up to date (%s).", version))
		return exitOK
	}

	if checkOnly {
		r.Info(fmt.Sprintf("A new abstr (%s) is available (current: %s).", latest, version))
		return exitOK
	}

	r.Status("Downloading abstr " + latest + "…")
	if err := selfupdate.Apply(ctx, latest); err != nil {
		if errors.Is(err, selfupdate.ErrUnmanagedInstall) {
			r.Error("abstr: " + err.Error())
			return exitRuntime
		}
		r.Error("abstr: upgrade failed: " + err.Error())
		return exitRuntime
	}

	r.Info("Upgraded abstr to " + latest + ".")
	return exitOK
}
