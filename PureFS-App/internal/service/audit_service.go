package service

import (
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
)

type AuditService struct {
	repo *repository.AuditLogRepo
}

func NewAuditService(repo *repository.AuditLogRepo) *AuditService {
	return &AuditService{repo: repo}
}

func (s *AuditService) Log(userID int64, action, detail, ip string) error {
	return s.repo.Create(&model.AuditLog{
		UserID: userID,
		Action: action,
		Detail: detail,
		IP:     ip,
	})
}

func (s *AuditService) List(limit, offset int) ([]*model.AuditLog, error) {
	return s.repo.List(limit, offset)
}
