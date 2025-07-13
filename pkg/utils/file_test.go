package utils

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCopyFile(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "copyfile_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create source file with test content
	srcPath := filepath.Join(tmpDir, "source.txt")
	testContent := "Hello, World!\nThis is a test file."
	err = os.WriteFile(srcPath, []byte(testContent), 0644)
	require.NoError(t, err)

	// Define destination path
	dstPath := filepath.Join(tmpDir, "destination.txt")

	// Test successful copy
	err = CopyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify file was copied correctly
	copiedContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(copiedContent))

	// Verify permissions are set to 0600
	info, err := os.Stat(dstPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestCopyFile_RelativePaths(t *testing.T) {
	// Test with relative paths (should fail)
	err := CopyFile("relative/path", "/absolute/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paths must be absolute")

	err = CopyFile("/absolute/path", "relative/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "paths must be absolute")
}

func TestCopyFile_NonExistentSource(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "copyfile_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	dstPath := filepath.Join(tmpDir, "destination.txt")

	err = CopyFile(srcPath, dstPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open source file")
}

func TestCopyFile_DestinationDirectoryCreation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "copyfile_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create source file
	srcPath := filepath.Join(tmpDir, "source.txt")
	testContent := "test content"
	err = os.WriteFile(srcPath, []byte(testContent), 0644)
	require.NoError(t, err)

	// Destination in nested directory that doesn't exist
	dstPath := filepath.Join(tmpDir, "nested", "dir", "destination.txt")

	err = CopyFile(srcPath, dstPath)
	assert.NoError(t, err)

	// Verify file was created and directory was created
	copiedContent, err := os.ReadFile(dstPath)
	require.NoError(t, err)
	assert.Equal(t, testContent, string(copiedContent))
}
