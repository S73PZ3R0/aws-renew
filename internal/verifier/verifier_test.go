package verifier

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

func TestVerifyFilesAllMatch(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "a.go", "package a")
	writeTempFile(t, dir, "b.go", "package b")

	checksum := fmt.Sprintf("%s  a.go\n%s  b.go\n",
		sha256Hex("package a"), sha256Hex("package b"))
	checksumPath := filepath.Join(dir, "CHECKSUM.asc")
	os.WriteFile(checksumPath, []byte(checksum), 0644)

	// Temporarily redirect checksumPath lookup
	orig := checksumPathFn
	checksumPathFn = func() string { return checksumPath }
	defer func() { checksumPathFn = orig }()

	matches, mismatches, err := VerifyFiles()
	if err != nil {
		t.Fatalf("VerifyFiles: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("unexpected mismatches: %v", mismatches)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}
}

func TestVerifyFilesMismatch(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "a.go", "package a")

	// Wrong hash on purpose
	checksum := fmt.Sprintf("%s  a.go\n", sha256Hex("wrong content"))
	checksumPath := filepath.Join(dir, "CHECKSUM.asc")
	os.WriteFile(checksumPath, []byte(checksum), 0644)

	orig := checksumPathFn
	checksumPathFn = func() string { return checksumPath }
	defer func() { checksumPathFn = orig }()

	_, mismatches, err := VerifyFiles()
	if err != nil {
		t.Fatalf("VerifyFiles: %v", err)
	}
	if len(mismatches) != 1 || mismatches[0] != "a.go" {
		t.Errorf("expected mismatch on a.go, got %v", mismatches)
	}
}

func TestVerifyFilesMissingFile(t *testing.T) {
	dir := t.TempDir()
	// Don't create the file — only put it in the checksum
	checksum := fmt.Sprintf("%s  missing.go\n", sha256Hex("content"))
	checksumPath := filepath.Join(dir, "CHECKSUM.asc")
	os.WriteFile(checksumPath, []byte(checksum), 0644)

	orig := checksumPathFn
	checksumPathFn = func() string { return checksumPath }
	defer func() { checksumPathFn = orig }()

	_, mismatches, err := VerifyFiles()
	if err != nil {
		t.Fatalf("VerifyFiles: %v", err)
	}
	if len(mismatches) != 1 {
		t.Errorf("expected 1 mismatch for missing file, got %v", mismatches)
	}
}

func TestVerifyFilesSkipsPGPHeaders(t *testing.T) {
	dir := t.TempDir()
	writeTempFile(t, dir, "a.go", "package a")

	// Simulate a clearsigned file with PGP headers
	checksum := "-----BEGIN PGP SIGNED MESSAGE-----\nHash: SHA512\n\n" +
		fmt.Sprintf("%s  a.go\n", sha256Hex("package a")) +
		"-----BEGIN PGP SIGNATURE-----\nfakesig\n-----END PGP SIGNATURE-----\n"
	checksumPath := filepath.Join(dir, "CHECKSUM.asc")
	os.WriteFile(checksumPath, []byte(checksum), 0644)

	orig := checksumPathFn
	checksumPathFn = func() string { return checksumPath }
	defer func() { checksumPathFn = orig }()

	matches, mismatches, err := VerifyFiles()
	if err != nil {
		t.Fatalf("VerifyFiles: %v", err)
	}
	if len(mismatches) != 0 {
		t.Errorf("PGP headers should be skipped, got mismatches: %v", mismatches)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
}
