package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
)

type ShareInfo struct {
	ID          int64      `json:"id"`
	FileID      int64      `json:"file_id"`
	FileName    string     `json:"file_name"`
	Token       string     `json:"token"`
	ExpiresAt   *time.Time `json:"expires_at"`
	MaxAccesses *int       `json:"max_accesses"`
	AccessCount int        `json:"access_count"`
	CanDownload bool       `json:"can_download"`
	IsActive    bool       `json:"is_active"`
	CreatedAt   time.Time  `json:"created_at"`
}

type ShareService struct {
	shareRepo *repository.ShareRepo
	fileRepo  *repository.FileRepo
}

func NewShareService(shareRepo *repository.ShareRepo, fileRepo *repository.FileRepo) *ShareService {
	return &ShareService{shareRepo: shareRepo, fileRepo: fileRepo}
}

func (s *ShareService) CreateShare(userID int64, req model.CreateShareRequest) (*model.Share, error) {
	f, err := s.fileRepo.GetByID(req.FileID)
	if err != nil {
		return nil, fmt.Errorf("file not found")
	}
	if f.UserID != userID {
		return nil, fmt.Errorf("permission denied")
	}

	token := generateShareToken()

	var expiresAt *time.Time
	if req.ExpiresIn != "" {
		d, err := time.ParseDuration(req.ExpiresIn)
		if err != nil {
			return nil, fmt.Errorf("invalid expires_in: %w", err)
		}
		t := time.Now().Add(d)
		expiresAt = &t
	}

	share := &model.Share{
		UserID:      userID,
		FileID:      req.FileID,
		Token:       token,
		Password:    req.Password,
		ExpiresAt:   expiresAt,
		MaxAccesses: req.MaxAccesses,
		CanDownload: req.CanDownload,
		IsActive:    true,
	}

	if err := s.shareRepo.Create(share); err != nil {
		return nil, fmt.Errorf("create share: %w", err)
	}

	return share, nil
}

func (s *ShareService) GetShare(token string) (*model.Share, *model.File, error) {
	share, f, err := s.validateShare(token)
	if err != nil {
		return nil, nil, err
	}

	// Legacy: increment access on view
	if err := s.shareRepo.IncrementAccess(share.ID); err != nil {
		return nil, nil, err
	}

	return share, f, nil
}

func (s *ShareService) ValidateShare(token string) (*model.Share, *model.File, error) {
	return s.validateShare(token)
}

func (s *ShareService) RecordAccess(token string) error {
	share, err := s.shareRepo.GetByToken(token)
	if err != nil {
		return fmt.Errorf("share not found")
	}
	return s.shareRepo.IncrementAccess(share.ID)
}

func (s *ShareService) validateShare(token string) (*model.Share, *model.File, error) {
	share, err := s.shareRepo.GetByToken(token)
	if err != nil {
		return nil, nil, fmt.Errorf("share not found")
	}

	if !share.IsActive {
		return nil, nil, fmt.Errorf("share is deactivated")
	}

	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, nil, fmt.Errorf("share has expired")
	}

	if share.MaxAccesses != nil && share.AccessCount >= *share.MaxAccesses {
		return nil, nil, fmt.Errorf("share access limit reached")
	}

	f, err := s.fileRepo.GetByID(share.FileID)
	if err != nil {
		return nil, nil, fmt.Errorf("shared file not found")
	}

	return share, f, nil
}

func (s *ShareService) ListShares(userID int64) ([]ShareInfo, error) {
	shares, err := s.shareRepo.ListByUser(userID)
	if err != nil {
		return nil, err
	}

	infos := make([]ShareInfo, 0, len(shares))
	for _, sh := range shares {
		info := ShareInfo{
			ID:          sh.ID,
			FileID:      sh.FileID,
			Token:       sh.Token,
			ExpiresAt:   sh.ExpiresAt,
			MaxAccesses: sh.MaxAccesses,
			AccessCount: sh.AccessCount,
			CanDownload: sh.CanDownload,
			IsActive:    sh.IsActive,
			CreatedAt:   sh.CreatedAt,
		}
		if f, err := s.fileRepo.GetByID(sh.FileID); err == nil {
			info.FileName = f.Name
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func (s *ShareService) DeactivateShare(userID int64, shareID int64) error {
	shares, err := s.shareRepo.ListByUser(userID)
	if err != nil {
		return err
	}
	for _, sh := range shares {
		if sh.ID == shareID {
			return s.shareRepo.Deactivate(shareID)
		}
	}
	return fmt.Errorf("share not found")
}

func generateShareToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}
