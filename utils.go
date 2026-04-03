package turbine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func computeApplicationVersion() string {
	hash, err := getBinaryHash()
	if err != nil {
		fmt.Printf("turbine: Failed to compute binary hash: %v\n", err)
		return ""
	}
	return hash
}

func getBinaryHash() (string, error) {
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return "", fmt.Errorf("resolve self path: %w", err)
	}
	fi, err := os.Lstat(execPath)
	if err != nil {
		return "", err
	}
	if !fi.Mode().IsRegular() {
		return "", fmt.Errorf("executable is not a regular file")
	}
	file, err := os.Open(execPath) // #nosec G304 -- opening our own executable
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
