package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/storage"
)

type IntegrityService struct {
	store storage.Storage
	cfg   *config.Config
}

func NewIntegrityService(store storage.Storage, cfg *config.Config) *IntegrityService {
	return &IntegrityService{store: store, cfg: cfg}
}

func (s *IntegrityService) RunCheck(rootPath string) (int, int, error) {
	if !s.cfg.Integrity.Enabled {
		return 0, 0, nil
	}

	var checked, failed int
	if err := s.checkDir(rootPath, &checked, &failed); err != nil {
		return checked, failed, err
	}

	return checked, failed, nil
}

func (s *IntegrityService) checkDir(dir string, checked, failed *int) error {
	entries, err := s.store.List(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir {
			if err := s.checkDir(entry.Path, checked, failed); err != nil {
				return err
			}
			continue
		}

		if _, err := s.computeFileHash(entry.Path); err != nil {
			*failed++
			continue
		}

		*checked++
	}

	return nil
}

func (s *IntegrityService) computeFileHash(path string) (string, error) {
	r, err := s.store.Open(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (s *IntegrityService) StartPeriodicCheck(rootPath string, interval time.Duration, stop chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			checked, failed, err := s.RunCheck(rootPath)
			if err != nil {
				fmt.Printf("Integrity check error: %v\n", err)
				continue
			}
			fmt.Printf("Integrity check: %d files checked, %d failed\n", checked, failed)
		case <-stop:
			return
		}
	}
}
