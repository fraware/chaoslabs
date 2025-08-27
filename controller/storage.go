package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// FileSystemStorage implements ExportStorage using local filesystem
type FileSystemStorage struct {
	basePath string
	baseURL  string
}

// NewFileSystemStorage creates a new filesystem storage
func NewFileSystemStorage(basePath, baseURL string) *FileSystemStorage {
	return &FileSystemStorage{
		basePath: basePath,
		baseURL:  baseURL,
	}
}

// Store stores data at the given key
func (fs *FileSystemStorage) Store(key string, data []byte) error {
	fullPath := filepath.Join(fs.basePath, key)

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := ioutil.WriteFile(fullPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file %s: %w", fullPath, err)
	}

	return nil
}

// Retrieve retrieves data for the given key
func (fs *FileSystemStorage) Retrieve(key string) ([]byte, error) {
	fullPath := filepath.Join(fs.basePath, key)

	data, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", fullPath, err)
	}

	return data, nil
}

// GetURL returns the download URL for the given key
func (fs *FileSystemStorage) GetURL(key string) (string, error) {
	// Clean the key to ensure it's URL-safe
	cleanKey := strings.ReplaceAll(key, "\\", "/")
	return fmt.Sprintf("%s/%s", fs.baseURL, cleanKey), nil
}

// Delete deletes the data at the given key
func (fs *FileSystemStorage) Delete(key string) error {
	fullPath := filepath.Join(fs.basePath, key)

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", fullPath, err)
	}

	return nil
}

// List lists all keys with the given prefix
func (fs *FileSystemStorage) List(prefix string) ([]string, error) {
	var keys []string

	prefixPath := filepath.Join(fs.basePath, prefix)

	err := filepath.Walk(prefixPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			// Convert back to key format
			relPath, err := filepath.Rel(fs.basePath, path)
			if err != nil {
				return err
			}

			// Normalize path separators
			key := strings.ReplaceAll(relPath, "\\", "/")
			keys = append(keys, key)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return keys, nil
}
