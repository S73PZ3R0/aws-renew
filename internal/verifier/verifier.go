package verifier

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type SignatureError struct{ msg string }

func (e *SignatureError) Error() string { return e.msg }

// checksumPathFn is a variable so tests can redirect it to a temp path.
var checksumPathFn = defaultChecksumPath

func defaultChecksumPath() string {
	const filename = "CHECKSUM.asc"
	
	// 1. Check current working directory first (common for dev/source runs)
	if _, err := os.Stat(filename); err == nil {
		return filename
	}

	// 2. Check next to the executable
	exe, err := os.Executable()
	if err == nil {
		path := filepath.Join(filepath.Dir(exe), filename)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default fallback
	return filename
}

// VerifySignature checks the GPG signature of the checksum file.
// If baseDir is provided, it looks for the checksum file there.
func VerifySignature(baseDir ...string) (bool, error) {
	path := checksumPathFn()
	if len(baseDir) > 0 {
		path = filepath.Join(baseDir[0], "CHECKSUM.asc")
	}
	
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	if _, err := exec.LookPath("gpg"); err != nil {
		return false, &SignatureError{"gpg not found in PATH"}
	}
	cmd := exec.Command("gpg", "--verify", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// VerifyFiles checks the hashes of files listed in the checksum file.
// If baseDir is provided, it verifies files relative to that directory.
func VerifyFiles(baseDir ...string) (matches []string, mismatches []string, err error) {
	path := checksumPathFn()
	targetDir := filepath.Dir(path)
	if len(baseDir) > 0 {
		path = filepath.Join(baseDir[0], "CHECKSUM.asc")
		targetDir = baseDir[0]
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-----") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}
		expectedHash, filePath := parts[0], parts[1]
		if _, err := hex.DecodeString(expectedHash); err != nil {
			continue
		}
		
		fullPath := filepath.Join(targetDir, filePath)

		fh, err := os.Open(fullPath)
		if err != nil {
			mismatches = append(mismatches, filePath)
			continue
		}
		h := sha256.New()
		if _, err := io.Copy(h, fh); err != nil {
			fh.Close()
			mismatches = append(mismatches, filePath)
			continue
		}
		fh.Close()
		actual := fmt.Sprintf("%x", h.Sum(nil))
		if actual == expectedHash {
			matches = append(matches, filePath)
		} else {
			mismatches = append(mismatches, filePath)
		}
	}
	return matches, mismatches, scanner.Err()
}
