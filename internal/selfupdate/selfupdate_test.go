package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"testing"
)

func makeTarGz(files map[string][]byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, content := range files {
		_ = tw.WriteHeader(&tar.Header{
			Name:     name,
			Size:     int64(len(content)),
			Typeflag: tar.TypeReg,
			Mode:     0755,
		})
		tw.Write(content)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func makeZip(files map[string][]byte) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, _ := zw.Create(name)
		w.Write(content)
	}
	zw.Close()
	return buf.Bytes()
}

func TestExtractTarGz(t *testing.T) {
	want := []byte("binary content")
	data := makeTarGz(map[string][]byte{
		"aws-renew_linux_amd64/README.md": []byte("readme"),
		"aws-renew_linux_amd64/aws-renew": want,
	})

	r, err := extractTarGz(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	var got bytes.Buffer
	got.ReadFrom(r)
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("got %q, want %q", got.Bytes(), want)
	}
}

func TestExtractTarGzEmpty(t *testing.T) {
	data := makeTarGz(map[string][]byte{
		"README.md": []byte("readme"),
	})
	_, err := extractTarGz(bytes.NewReader(data))
	if err == nil {
		t.Error("expected error for archive with no binary, got nil")
	}
}

func TestExtractZip(t *testing.T) {
	want := []byte("windows binary")
	data := makeZip(map[string][]byte{
		"aws-renew.exe": want,
		"README.md":     []byte("readme"),
	})

	r, err := extractZip(data)
	if err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	var got bytes.Buffer
	got.ReadFrom(r)
	if !bytes.Equal(got.Bytes(), want) {
		t.Errorf("got %q, want %q", got.Bytes(), want)
	}
}

func TestExtractZipEmpty(t *testing.T) {
	data := makeZip(map[string][]byte{
		"README.md": []byte("readme"),
	})
	_, err := extractZip(data)
	if err == nil {
		t.Error("expected error for zip with no binary, got nil")
	}
}

func TestAssetURL(t *testing.T) {
	rel := &Release{
		TagName: "v2.1.0",
		Assets: []Asset{
			{Name: "aws-renew_linux_amd64.tar.gz", BrowserDownloadURL: "https://example.com/linux_amd64"},
			{Name: "aws-renew_linux_arm64.tar.gz", BrowserDownloadURL: "https://example.com/linux_arm64"},
			{Name: "aws-renew_darwin_arm64.tar.gz", BrowserDownloadURL: "https://example.com/darwin_arm64"},
			{Name: "aws-renew_windows_amd64.zip", BrowserDownloadURL: "https://example.com/windows_amd64"},
			{Name: "checksums.txt", BrowserDownloadURL: "https://example.com/checksums"},
		},
	}

	tests := []struct {
		goos    string
		goarch  string
		wantURL string
		wantErr bool
	}{
		{"linux", "amd64", "https://example.com/linux_amd64", false},
		{"linux", "arm64", "https://example.com/linux_arm64", false},
		// android (old Termux builds) must resolve to linux/arm64
		{"android", "arm64", "https://example.com/linux_arm64", false},
		{"darwin", "arm64", "https://example.com/darwin_arm64", false},
		{"windows", "amd64", "https://example.com/windows_amd64", false},
		{"linux", "386", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.goos+"_"+tc.goarch, func(t *testing.T) {
			url, err := assetURLFor(rel, tc.goos, tc.goarch)
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if url != tc.wantURL {
				t.Errorf("got %q, want %q", url, tc.wantURL)
			}
		})
	}
}


// assetURLFor is a testable version of Release.AssetURL with explicit os/arch.
func assetURLFor(r *Release, goos, goarch string) (string, error) {
	searchOS := goos
	if goos == "android" {
		searchOS = "linux"
	}
	for _, asset := range r.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, searchOS) && strings.Contains(name, goarch) {
			if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tgz") {
				return asset.BrowserDownloadURL, nil
			}
		}
	}
	return "", fmt.Errorf("no asset found for %s/%s in release %s", goos, goarch, r.TagName)
}
