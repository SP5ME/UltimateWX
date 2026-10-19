package updater

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"
)

func TestExtractBinaryRejectsUnsafeMembersAndFindsExecutable(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, entry := range []struct {
		name string
		data []byte
	}{{"../escape", []byte("bad")}, {"ultimatewx", []byte("binary")}} {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: 0755, Size: int64(len(entry.data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := extractBinary(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "binary" {
		t.Fatalf("got %q", got)
	}
}

func TestExtractBinaryRequiresExpectedFile(t *testing.T) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	_ = tw.WriteHeader(&tar.Header{Name: "other", Mode: 0755, Size: 1, Typeflag: tar.TypeReg})
	_, _ = tw.Write([]byte("x"))
	_ = tw.Close()
	_ = gz.Close()
	if _, err := extractBinary(buf.Bytes()); err == nil {
		t.Fatal("expected missing binary to fail")
	}
}

func TestLatestReleaseUsesSemanticVersionNotGitHubListOrder(t *testing.T) {
	releases := []Release{
		{TagName: "v0.0.3-prealpha.9"},
		{TagName: "v0.0.3-prealpha.8"},
		{TagName: "v0.0.3-prealpha.10"},
		{TagName: "v0.0.3-prealpha.99", Draft: true},
	}
	got, ok := latestSemverRelease(releases)
	if !ok || got.TagName != "v0.0.3-prealpha.10" {
		t.Fatalf("latest release = %q, %v; want v0.0.3-prealpha.10", got.TagName, ok)
	}
}

func TestCompareVersionsSemverPrecedence(t *testing.T) {
	for _, test := range []struct {
		a, b string
		want int
	}{
		{"v0.0.3-prealpha.10", "v0.0.3-prealpha.9", 1},
		{"v0.0.3", "v0.0.3-prealpha.10", 1},
		{"v0.0.3-alpha.1", "v0.0.3-alpha.beta", -1},
		{"v0.0.3-alpha", "v0.0.3-alpha.1", -1},
	} {
		got, ok := compareVersions(test.a, test.b)
		if !ok || got != test.want {
			t.Errorf("compareVersions(%q, %q) = %d, %v; want %d, true", test.a, test.b, got, ok, test.want)
		}
	}
}
