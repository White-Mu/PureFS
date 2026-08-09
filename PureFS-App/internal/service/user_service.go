package service

import (
	"fmt"

	"github.com/purefs/purefs/internal/auth"
	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/pkg/jwtutil"
)

type UserService struct {
	userRepo *repository.UserRepo
	cfg      *config.Config
}

func NewUserService(userRepo *repository.UserRepo, cfg *config.Config) *UserService {
	return &UserService{userRepo: userRepo, cfg: cfg}
}

func (s *UserService) Register(req model.RegisterUserRequest) (*model.User, error) {
	// Validate password strength against configured policy
	pp := s.cfg.Auth.PasswordPolicy
	if err := auth.ValidatePasswordStrength(req.Password, pp.MinLength, pp.RequireUpper, pp.RequireLower, pp.RequireDigit, pp.RequireSpecial); err != nil {
		return nil, fmt.Errorf("password validation: %w", err)
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &model.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		Role:         "user",
		StorageQuota: 10 << 30,
		IsActive:     true,
		RootDir:      fmt.Sprintf("/users/%s", req.Username),
	}

	if err := s.userRepo.Create(u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return u, nil
}

func (s *UserService) Login(req model.LoginRequest) (*model.LoginResponse, error) {
	u, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	if !auth.CheckPassword(req.Password, u.PasswordHash) {
		return nil, fmt.Errorf("invalid username or password")
	}

	if !u.IsActive {
		return nil, fmt.Errorf("account is disabled")
	}

	if u.TOTPEnabled {
		if req.TOTPCode == "" {
			return &model.LoginResponse{TOTPRequired: true}, nil
		}
		if !auth.ValidateTOTP(u.TOTPSecret, req.TOTPCode) {
			return nil, fmt.Errorf("invalid 2FA code")
		}
	}

	token, err := jwtutil.GenerateToken(u.ID, u.Username, u.Role, s.cfg.Auth.JWTSecret, s.cfg.Auth.JWTExpiry)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &model.LoginResponse{
		Token: token,
		User:  *u,
	}, nil
}

func (s *UserService) GetUser(id int64) (*model.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *UserService) ListUsers() ([]*model.User, error) {
	return s.userRepo.List()
}

func (s *UserService) SetupTOTP(userID int64) (string, string, error) {
	u, err := s.userRepo.GetByID(userID)
	if err != nil {
		return "", "", err
	}

	secret, err := auth.GenerateTOTPSecret()
	if err != nil {
		return "", "", err
	}

	u.TOTPSecret = secret
	uri := auth.GenerateTOTPURI(secret, u.Username, "PureFS")

	if err := s.userRepo.Update(u); err != nil {
		return "", "", err
	}

	return secret, uri, nil
}

func (s *UserService) EnableTOTP(userID int64, code string) error {
	u, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	if !auth.ValidateTOTP(u.TOTPSecret, code) {
		return fmt.Errorf("invalid verification code")
	}

	u.TOTPEnabled = true
	return s.userRepo.Update(u)
}

func (s *UserService) DisableTOTP(userID int64) error {
	u, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	u.TOTPEnabled = false
	u.TOTPSecret = ""
	return s.userRepo.Update(u)
}
