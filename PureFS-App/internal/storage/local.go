package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type LocalStorage struct {
	basePath string
}

func NewLocalStorage(basePath string) (*LocalStorage, error) {
	abs, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("resolve base path: %w", err)
	}
	if err := os.MkdirAll(abs, 0755); err != nil {
		return nil, fmt.Errorf("create base dir: %w", err)
	}
	return &LocalStorage{basePath: abs}, nil
}

func (s *LocalStorage) fullPath(logicalPath string) string {
	return filepath.Join(s.basePath, filepath.Clean(logicalPath))
}

func (s *LocalStorage) Open(path string) (io.ReadCloser, error) {
	return os.Open(s.fullPath(path))
}

func (s *LocalStorage) Create(path string) (io.WriteCloser, error) {
	full := s.fullPath(path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		return nil, err
	}
	return os.Create(full)
}

func (s *LocalStorage) Delete(path string) error {
	return os.RemoveAll(s.fullPath(path))
}

func (s *LocalStorage) Stat(path string) (*FileInfo, error) {
	info, err := os.Stat(s.fullPath(path))
	if err != nil {
		return nil, err
	}
	return &FileInfo{
		Path:  path,
		Size:  info.Size(),
		IsDir: info.IsDir(),
	}, nil
}

func (s *LocalStorage) List(dir string) ([]*FileInfo, error) {
	entries, err := os.ReadDir(s.fullPath(dir))
	if err != nil {
		return nil, err
	}
	var result []*FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, &FileInfo{
			Path:  filepath.Join(dir, e.Name()),
			Size:  info.Size(),
			IsDir: e.IsDir(),
		})
	}
	return result, nil
}

func (s *LocalStorage) Mkdir(path string) error {
	return os.MkdirAll(s.fullPath(path), 0755)
}

func (s *LocalStorage) Rename(oldPath, newPath string) error {
	return os.Rename(s.fullPath(oldPath), s.fullPath(newPath))
}

func (s *LocalStorage) Copy(srcPath, dstPath string) error {
	src := s.fullPath(srcPath)
	dst := s.fullPath(dstPath)
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return copyFile(src, dst)
}

func (s *LocalStorage) Exists(path string) (bool, error) {
	_, err := os.Stat(s.fullPath(path))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *LocalStorage) RealPath(logicalPath string) string {
	return s.fullPath(logicalPath)
}

func (s *LocalStorage) ComputeSHA256(path string) (string, error) {
	f, err := os.Open(s.fullPath(path))
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return dstFile.Sync()
}

// Ensure LocalStorage implements Storage.
var _ Storage = (*LocalStorage)(nil)
