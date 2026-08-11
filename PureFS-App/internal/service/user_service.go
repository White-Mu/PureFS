package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/purefs/purefs/internal/auth"
	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/pkg/jwtutil"
)

type UserService struct {
	userRepo *repository.UserRepo
	cfg      *config.Config
	auditSvc *AuditService
}

func NewUserService(userRepo *repository.UserRepo, cfg *config.Config) *UserService {
	return &UserService{userRepo: userRepo, cfg: cfg}
}

// SetAuditService sets the audit logging service.
func (s *UserService) SetAuditService(auditSvc *AuditService) {
	s.auditSvc = auditSvc
}

func (s *UserService) logAudit(userID int64, action, detail string) {
	if s.auditSvc == nil {
		return
	}
	_ = s.auditSvc.Log(userID, action, detail, "")
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

	s.logAudit(u.ID, "register", "user registered: "+req.Username)
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

	s.logAudit(u.ID, "login", "user logged in")
	return &model.LoginResponse{
		Token: token,
		User:  *u,
	}, nil
}

func (s *UserService) GetUser(id int64) (*model.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *UserService) RefreshToken(userID int64) (*model.LoginResponse, error) {
	u, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !u.IsActive {
		return nil, fmt.Errorf("account is disabled")
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

// AdminCreateUser creates a new user as an admin. It validates password policy
// and checks for duplicate usernames before creating.
func (s *UserService) AdminCreateUser(username, email, password, role string) (*model.User, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("username and password are required")
	}

	// Check for duplicate
	if existing, _ := s.userRepo.GetByUsername(username); existing != nil {
		return nil, fmt.Errorf("username already exists")
	}

	// Validate password policy
	policy := s.cfg.Auth.PasswordPolicy
	if len(password) < policy.MinLength {
		return nil, fmt.Errorf("password must be at least %d characters", policy.MinLength)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &model.User{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		IsActive:     true,
		StorageQuota: 10 << 30, // 10 GB default
	}

	if err := s.userRepo.Create(u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return u, nil
}

// ToggleUserActive enables or disables a user account.
func (s *UserService) ToggleUserActive(userID int64, active bool) error {
	u, err := s.userRepo.GetByID(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	u.IsActive = active
	return s.userRepo.Update(u)
}

// ForgotPassword generates a reset token for the user and returns it.
// In production, the token should be emailed; for self-hosted use, the
// token is returned in the response so the admin can relay it.
func (s *UserService) ForgotPassword(username string) (string, error) {
	u, err := s.userRepo.GetByUsername(username)
	if err != nil {
		// Don't reveal whether the user exists
		return "", nil
	}

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)
	expires := time.Now().Add(1 * time.Hour)

	if err := s.userRepo.SetResetToken(u.ID, token, expires); err != nil {
		return "", fmt.Errorf("set reset token: %w", err)
	}

	return token, nil
}

// ResetPassword validates a reset token and sets a new password.
func (s *UserService) ResetPassword(token, newPassword string) error {
	u, err := s.userRepo.GetByResetToken(token)
	if err != nil {
		return fmt.Errorf("invalid or expired reset token")
	}

	policy := s.cfg.Auth.PasswordPolicy
	if len(newPassword) < policy.MinLength {
		return fmt.Errorf("password must be at least %d characters", policy.MinLength)
	}

	hash, err := auth.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	u.PasswordHash = hash
	u.ResetToken = ""
	u.ResetTokenExpires = nil

	if err := s.userRepo.Update(u); err != nil {
		return fmt.Errorf("update user: %w", err)
	}

	// Clear the reset token separately
	if err := s.userRepo.SetResetToken(u.ID, "", time.Time{}); err != nil {
		return fmt.Errorf("clear reset token: %w", err)
	}

	return nil
}
