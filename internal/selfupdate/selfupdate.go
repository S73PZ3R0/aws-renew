package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/inconshreveable/go-update"
	"github.com/S73PZ3R0/aws-renew/internal/verifier"
)

const githubReleasesURL = "https://api.github.com/repos/S73PZ3R0/aws-renew/releases"

type Release struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func LatestRelease() (*Release, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(githubReleasesURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()
	
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse releases: %w", err)
	}
	if len(releases) == 0 {
		return nil, fmt.Errorf("no releases found")
	}
	// Return the most recent release (index 0)
	return &releases[0], nil
}

func (r *Release) AssetURL() (string, error) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	searchOS := osName
	// GOOS=android binaries (old Termux builds) should update via linux/arm64.
	if osName == "android" {
		searchOS = "linux"
	}

	for _, asset := range r.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, searchOS) && strings.Contains(name, arch) {
			if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip") || strings.HasSuffix(name, ".tgz") {
				return asset.BrowserDownloadURL, nil
			}
		}
	}

	return "", fmt.Errorf("no asset found for %s/%s in release %s", osName, arch, r.TagName)
}

func DownloadChecksum(version, targetPath string) error {
	rel, err := getReleaseByTag("v" + version)
	if err != nil {
		rel, err = LatestRelease()
		if err != nil {
			return err
		}
	}

	var downloadURL string
	for _, asset := range rel.Assets {
		if asset.Name == "CHECKSUM.asc" {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("CHECKSUM.asc not found in release %s", rel.TagName)
	}

	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func getReleaseByTag(tag string) (*Release, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(githubReleasesURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}
	for _, rel := range releases {
		if rel.TagName == tag {
			return &rel, nil
		}
	}
	return nil, fmt.Errorf("release %s not found", tag)
}

func Apply(url string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read update: %w", err)
	}

	// Create a temporary directory to verify the update
	tmpDir, err := os.MkdirTemp("", "aws-renew-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract all files for verification
	switch {
	case strings.HasSuffix(url, ".tar.gz") || strings.HasSuffix(url, ".tgz"):
		if err := unpackTarGz(bytes.NewReader(body), tmpDir); err != nil {
			return fmt.Errorf("failed to unpack tar.gz: %w", err)
		}
	case strings.HasSuffix(url, ".zip"):
		if err := unpackZip(body, tmpDir); err != nil {
			return fmt.Errorf("failed to unpack zip: %w", err)
		}
	default:
		return fmt.Errorf("unsupported archive format: %s", url)
	}

	// 1. Mandatory Security Check: Verify GPG Signature of the author (S73PZ3R0)
	ok, err := verifier.VerifySignature(tmpDir)
	if err != nil {
		return fmt.Errorf("SECURITY_ALERT: Update signature verification failed: %w", err)
	}
	if !ok {
		return fmt.Errorf("SECURITY_ALERT: Update binary is UNSIGNED or signature is INVALID")
	}

	// 2. Mandatory Integrity Check: Verify all file hashes
	_, mismatches, err := verifier.VerifyFiles(tmpDir)
	if err != nil {
		return fmt.Errorf("INTEGRITY_ALERT: Hash verification error: %w", err)
	}
	if len(mismatches) > 0 {
		return fmt.Errorf("INTEGRITY_ALERT: Update contains tampered files: %v", mismatches)
	}

	// 3. Find the executable binary in the verified files
	var binaryPath string
	entries, _ := os.ReadDir(tmpDir)
	for _, entry := range entries {
		name := strings.ToLower(entry.Name())
		if !entry.IsDir() && (name == "aws-renew" || name == "aws-renew.exe") {
			binaryPath = filepath.Join(tmpDir, entry.Name())
			break
		}
	}
	if binaryPath == "" {
		return fmt.Errorf("binary not found in verified update")
	}

	f, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Errorf("failed to open verified binary: %w", err)
	}
	defer f.Close()

	// os.Executable() is not implemented on GOOS=android (Termux).
	// Resolve the target path via os.Args[0] as a fallback so the update
	// can still be applied from an android-compiled binary.
	applyOpts := update.Options{}
	if _, err := os.Executable(); err != nil {
		execPath := os.Args[0]
		if !filepath.IsAbs(execPath) {
			if wd, werr := os.Getwd(); werr == nil {
				execPath = filepath.Join(wd, execPath)
			}
		}
		applyOpts.TargetPath = execPath
	}

	return update.Apply(f, applyOpts)
}

// extractTarGz finds the aws-renew binary inside a tar.gz archive and returns
// its content as a reader.
func extractTarGz(r io.Reader) (io.Reader, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		base := strings.ToLower(filepath.Base(hdr.Name))
		if base == "aws-renew" || base == "aws-renew.exe" {
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, tr); err != nil {
				return nil, err
			}
			return &buf, nil
		}
	}
	return nil, fmt.Errorf("no binary found in archive")
}

// extractZip finds the aws-renew binary inside a zip archive and returns its
// content as a reader.
func extractZip(data []byte) (io.Reader, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		base := strings.ToLower(filepath.Base(f.Name))
		if base == "aws-renew" || base == "aws-renew.exe" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				return nil, err
			}
			return &buf, nil
		}
	}
	return nil, fmt.Errorf("no binary found in zip archive")
}

// unpackTarGz extracts all files from a tar.gz archive to dst, preserving
// directory structure. GoReleaser archives have no top-level wrapper directory.
func unpackTarGz(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		relPath := hdr.Name
		if relPath == "" {
			continue
		}
		target := filepath.Join(dst, relPath)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)+string(os.PathSeparator)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}

// unpackZip extracts all files from a zip archive to dst, preserving directory
// structure. GoReleaser archives have no top-level wrapper directory.
func unpackZip(data []byte, dst string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		relPath := f.Name
		if relPath == "" {
			continue
		}
		target := filepath.Join(dst, relPath)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dst)+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, f.Mode())
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			out.Close()
			rc.Close()
			return err
		}
		out.Close()
		rc.Close()
	}
	return nil
}

