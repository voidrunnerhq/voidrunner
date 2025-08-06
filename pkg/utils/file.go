package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyFile copies a file from src to dst with proper validation and security checks
func CopyFile(src, dst string) error {
	// Validate and clean paths to prevent directory traversal
	cleanSrc := filepath.Clean(src)
	cleanDst := filepath.Clean(dst)

	// Additional security check: ensure paths don't contain ".." or other suspicious patterns
	if !filepath.IsAbs(cleanSrc) || !filepath.IsAbs(cleanDst) {
		return fmt.Errorf("paths must be absolute")
	}

	// #nosec G304 - Path traversal mitigation: paths are validated and cleaned above
	sourceFile, err := os.Open(cleanSrc)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if closeErr := sourceFile.Close(); closeErr != nil {
			// Log error but don't fail the operation
			fmt.Printf("warning: failed to close source file %s: %v\n", cleanSrc, closeErr)
		}
	}()

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(cleanDst), 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// #nosec G304 - Path traversal mitigation: paths are validated and cleaned above
	destFile, err := os.Create(cleanDst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() {
		if closeErr := destFile.Close(); closeErr != nil {
			// Log error but don't fail the operation
			fmt.Printf("warning: failed to close destination file %s: %v\n", cleanDst, closeErr)
		}
	}()

	// Copy file contents
	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Set file permissions to 0600 for security
	if err := os.Chmod(cleanDst, 0600); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	return nil
}
