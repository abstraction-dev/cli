package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v1.2.3", "v1.2.4", true},
		{"1.2.3", "v1.2.4", true},   // bare current is normalized
		{"abstr 1.2.3", "v2.0.0", true}, // "abstr "-prefixed current
		{"v1.2.3", "v1.2.3", false},
		{"v1.2.4", "v1.2.3", false}, // downgrade
		{"dev", "v1.2.3", false},    // local build never upgrades
		{"v1.2.3", "garbage", false},
	}
	for _, c := range cases {
		if got := IsNewer(c.current, c.latest); got != c.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestVerifyChecksum(t *testing.T) {
	archive := []byte("pretend this is a tarball")
	sum := sha256.Sum256(archive)
	asset := "abstr_linux_amd64.tar.gz"
	manifest := []byte(
		"deadbeef  abstr_darwin_arm64.tar.gz\n" +
			hex.EncodeToString(sum[:]) + "  " + asset + "\n",
	)

	if err := verifyChecksum(archive, manifest, asset); err != nil {
		t.Fatalf("verifyChecksum matching: %v", err)
	}

	if err := verifyChecksum([]byte("tampered"), manifest, asset); err == nil {
		t.Error("verifyChecksum should fail on mismatch")
	}

	if err := verifyChecksum(archive, manifest, "abstr_windows_amd64.zip"); err == nil {
		t.Error("verifyChecksum should fail when asset absent from manifest")
	}
}

func TestExtractFromTarGz(t *testing.T) {
	want := []byte("#!/binary payload")
	archive := makeTarGz(t, map[string][]byte{
		"README.md": []byte("docs"),
		"abstr":     want,
	})

	got, err := extractBinary(archive, "abstr_linux_amd64.tar.gz")
	if err != nil {
		t.Fatalf("extractBinary: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("extracted %q, want %q", got, want)
	}
}

func TestExtractFromTarGzMissing(t *testing.T) {
	archive := makeTarGz(t, map[string][]byte{"README.md": []byte("docs")})
	if _, err := extractBinary(archive, "abstr_linux_amd64.tar.gz"); err == nil {
		t.Error("extractBinary should fail when binary absent")
	}
}

func makeTarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, data := range files {
		hdr := &tar.Header{Name: name, Mode: 0o755, Size: int64(len(data)), Typeflag: tar.TypeReg}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
