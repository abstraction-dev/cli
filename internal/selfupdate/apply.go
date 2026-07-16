package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// maxAssetBytes caps how much we will read for a release asset, as a guard
// against a pathological response. The binary is a few MB.
const maxAssetBytes = 200 << 20 // 200 MiB

// downloadTimeout bounds a single asset download end-to-end.
const downloadTimeout = 2 * time.Minute

// downloadClient fetches release assets. Its Timeout is a backstop; callers also
// pass a context deadline.
var downloadClient = &http.Client{Timeout: downloadTimeout}

// ErrUnmanagedInstall is returned when the running binary lives somewhere we
// must not overwrite — a package-manager prefix (Homebrew, Nix) or a directory
// we cannot write to. The caller should tell the user to use their installer.
var ErrUnmanagedInstall = errors.New("binary is not in a self-updatable location")

// Apply downloads the release archive for tag, verifies it against the
// published sha256sums.txt, and atomically replaces the running executable.
// Self-upgrade is Unix-only; on Windows, install manually or use WSL.
func Apply(ctx context.Context, tag string) error {
	if runtime.GOOS == "windows" {
		return errors.New("self-upgrade is not supported on Windows — download the latest release manually or use WSL")
	}

	exe, err := resolveExecutable()
	if err != nil {
		return err
	}
	if err := checkWritable(exe); err != nil {
		return err
	}

	asset := assetName()
	base := "https://github.com/" + repoSlug + "/releases/download/" + tag + "/"

	archive, err := download(ctx, base+asset)
	if err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}

	sums, err := download(ctx, base+"sha256sums.txt")
	if err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := verifyChecksum(archive, sums, asset); err != nil {
		return err
	}

	binary, err := extractBinary(archive)
	if err != nil {
		return err
	}

	return replaceBinary(exe, binary)
}

// resolveExecutable returns the real on-disk path of the running binary, with
// symlinks resolved so that package-manager installs (which symlink into a bin
// dir from a versioned cellar) are detected by their true location.
func resolveExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return exe, nil
}

// checkWritable refuses installs we must not touch: package-manager prefixes
// (their upgrades are owned by the manager) and directories we cannot write to.
func checkWritable(exe string) error {
	lower := strings.ToLower(exe)
	for _, marker := range []string{"/cellar/", "/opt/homebrew/", "/nix/store/", "/var/lib/flatpak/"} {
		if strings.Contains(lower, marker) {
			return fmt.Errorf("%w: %s looks like a package-manager install; upgrade with your package manager instead", ErrUnmanagedInstall, exe)
		}
	}

	dir := filepath.Dir(exe)
	probe, err := os.CreateTemp(dir, ".abstr-update-*")
	if err != nil {
		return fmt.Errorf("%w: cannot write to %s: %v", ErrUnmanagedInstall, dir, err)
	}
	name := probe.Name()
	probe.Close()
	os.Remove(name)
	return nil
}

// replaceBinary installs newBinary at target atomically: it writes to a temp
// file in the same directory (so os.Rename stays on one filesystem and is
// atomic), preserves the current file's permissions, then renames over the
// target. The old binary is never removed until the rename succeeds, so a
// failure leaves the existing install untouched — no rollback needed. Replacing
// a running binary is safe on Unix: the process keeps its open inode.
func replaceBinary(target string, newBinary []byte) error {
	perm := os.FileMode(0o755)
	if info, err := os.Stat(target); err == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, ".abstr-new-*")
	if err != nil {
		return fmt.Errorf("create temp binary: %w", err)
	}
	tmpName := tmp.Name()
	// Remove the temp file if we don't successfully rename it into place.
	defer os.Remove(tmpName)

	if _, err := tmp.Write(newBinary); err != nil {
		tmp.Close()
		return fmt.Errorf("write new binary: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close new binary: %w", err)
	}

	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("replace %s: %w", target, err)
	}
	return nil
}

// assetName is the release archive for the current platform (see
// .goreleaser.yaml). Every supported platform ships as tar.gz.
func assetName() string {
	return fmt.Sprintf("%s_%s_%s.tar.gz", binaryName, runtime.GOOS, runtime.GOARCH)
}

// download fetches a URL fully into memory, erroring if the response exceeds
// maxAssetBytes rather than silently truncating.
func download(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := downloadClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %s", resp.Status)
	}

	// Read one byte past the cap so we can distinguish "exactly at cap" from
	// "over cap" and fail loudly on an oversized (or unbounded) response.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxAssetBytes {
		return nil, fmt.Errorf("response exceeds %d bytes", maxAssetBytes)
	}
	return body, nil
}

// verifyChecksum confirms archive's sha256 matches the entry for asset in a
// sha256sums.txt manifest. Lines are "<hex>  <filename>"; a leading '*' on the
// filename (sha256sum's binary-mode output) is tolerated.
func verifyChecksum(archive, sums []byte, asset string) error {
	var want string
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == asset {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("no checksum entry for %s", asset)
	}

	sum := sha256.Sum256(archive)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, want) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", asset, want, got)
	}
	return nil
}

// extractBinary pulls the abstr binary out of the release tarball. It sits at
// the archive root.
func extractBinary(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if path.Base(hdr.Name) == binaryName && hdr.Typeflag == tar.TypeReg {
			return io.ReadAll(io.LimitReader(tr, maxAssetBytes))
		}
	}
	return nil, fmt.Errorf("archive did not contain a %q binary", binaryName)
}
