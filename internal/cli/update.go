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

// checkResult is what a background release check reports back to finish.
type checkResult struct {
	ran bool   // the network check actually executed this run
	tag string // resolved latest tag; "" when the check was skipped or failed
}

// updateCheck is a background release check started alongside the main task. Its
// result is consumed once, at the end of the run, by finish.
type updateCheck struct {
	cfg    *config.Config
	result chan checkResult
}

// startUpdateCheck kicks off a throttled, concurrent release check. It returns a
// no-op check when updates are disabled (dev build, CI, opt-out, non-TTY) or
// when the last check was recent — in which case the run still acts on the
// cached LatestSeen. The check runs concurrently with the user's task so its
// latency is hidden behind work that is already happening.
func startUpdateCheck(ctx context.Context, cfg *config.Config) *updateCheck {
	uc := &updateCheck{cfg: cfg, result: make(chan checkResult, 1)}

	if !updatesEnabled() || !checkDue(cfg) {
		uc.result <- checkResult{ran: false}
		return uc
	}

	go func() {
		cctx, cancel := context.WithTimeout(ctx, updateCheckTimeout)
		defer cancel()
		tag, err := selfupdate.LatestVersion(cctx)
		if err != nil {
			// Fail silent — background checks never nag on error — but report
			// that the check ran so its timestamp still gets stamped.
			uc.result <- checkResult{ran: true, tag: ""}
			return
		}
		uc.result <- checkResult{ran: true, tag: tag}
	}()
	return uc
}

// finish consumes the check result, persists the cache, and either prints an
// upgrade notice or (when auto-upgrade is enabled) applies the update in place.
// It is safe to call once per run, after the main task completes.
func finish(ctx context.Context, uc *updateCheck) {
	res := <-uc.result

	// Whenever a check actually ran, stamp the timestamp — even on failure — so a
	// persistent network error doesn't make every subsequent run re-check.
	// Reload from disk first so we don't clobber fields the task wrote, and so
	// the decision below sees the latest persisted state rather than the config
	// we loaded at process start.
	cfg := uc.cfg
	if res.ran {
		if fresh, err := config.Load(uc.cfg.FilePath()); err == nil {
			fresh.LastUpdateCheck = nowUTC()
			if res.tag != "" {
				fresh.LatestSeen = res.tag
			}
			_ = fresh.Save()
			cfg = fresh
		}
	}

	// Decide against the newest tag we know about: this run's result if we have
	// one, else what the (freshly reloaded) config has cached.
	newest := res.tag
	if newest == "" {
		newest = cfg.LatestSeen
	}
	if newest == "" || !selfupdate.IsNewer(version, newest) {
		return
	}

	r := newRenderer()
	if cfg.AutoUpgrade {
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

	// Bound the whole operation so a stalled connection can't hang the command.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

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
		if isDev {
			r.Info(fmt.Sprintf("Running a dev build; latest release is %s.", latest))
		} else {
			r.Info(fmt.Sprintf("A new abstr (%s) is available (current: %s).", latest, version))
		}
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
