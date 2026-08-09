package model

import "time"

type FileType string

const (
	FileTypeFile    FileType = "file"
	FileTypeDir     FileType = "directory"
	FileTypeSymlink FileType = "symlink"
)

type File struct {
	ID            int64     `json:"id" db:"id"`
	UserID        int64     `json:"user_id" db:"user_id"`
	ParentID      *int64    `json:"parent_id" db:"parent_id"`
	Name          string    `json:"name" db:"name"`
	Path          string    `json:"path" db:"path"`
	RealPath      string    `json:"real_path" db:"real_path"`
	FileType      FileType  `json:"file_type" db:"file_type"`
	MimeType      string    `json:"mime_type" db:"mime_type"`
	Size          int64     `json:"size" db:"size"`
	SHA256        string    `json:"sha256" db:"sha256"`
	IsPinned      bool      `json:"is_pinned" db:"is_pinned"`
	IsFavorite    bool      `json:"is_favorite" db:"is_favorite"`
	IsEncrypted   bool      `json:"is_encrypted" db:"is_encrypted"`
	DEKCiphertext string    `json:"dek_ciphertext" db:"dek_ciphertext"`
	KEKVersion    int64     `json:"kek_version" db:"kek_version"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type User struct {
	ID            int64     `json:"id" db:"id"`
	Username      string    `json:"username" db:"username"`
	Email         string    `json:"email" db:"email"`
	PasswordHash  string    `json:"-" db:"password_hash"`
	Role          string    `json:"role" db:"role"` // "admin", "user"
	TOTPSecret    string    `json:"-" db:"totp_secret"`
	TOTPEnabled   bool      `json:"totp_enabled" db:"totp_enabled"`
	StorageQuota  int64     `json:"storage_quota" db:"storage_quota"`
	StorageUsed   int64     `json:"storage_used" db:"storage_used"`
	RootDir       string    `json:"root_dir" db:"root_dir"`
	IsActive      bool      `json:"is_active" db:"is_active"`
	SSHPublicKey  string    `json:"-" db:"ssh_public_key"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

type Share struct {
	ID          int64     `json:"id" db:"id"`
	UserID      int64     `json:"user_id" db:"user_id"`
	FileID      int64     `json:"file_id" db:"file_id"`
	Token       string    `json:"token" db:"token"`
	Password    string    `json:"-" db:"password"`
	ExpiresAt   *time.Time `json:"expires_at" db:"expires_at"`
	MaxAccesses *int      `json:"max_accesses" db:"max_accesses"`
	AccessCount int       `json:"access_count" db:"access_count"`
	CanDownload bool      `json:"can_download" db:"can_download"`
	IsActive    bool      `json:"is_active" db:"is_active"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type Permission struct {
	ID       int64  `json:"id" db:"id"`
	UserID   int64  `json:"user_id" db:"user_id"`
	FilePath string `json:"file_path" db:"file_path"`
	Perm     string `json:"perm" db:"perm"` // "read", "write", "admin"
}

type AuditLog struct {
	ID        int64     `json:"id" db:"id"`
	UserID    int64     `json:"user_id" db:"user_id"`
	Action    string    `json:"action" db:"action"`
	Detail    string    `json:"detail" db:"detail"`
	IP        string    `json:"ip" db:"ip"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type IntegrityRecord struct {
	ID           int64     `json:"id" db:"id"`
	FilePath     string    `json:"file_path" db:"file_path"`
	SHA256       string    `json:"sha256" db:"sha256"`
	FileSize     int64     `json:"file_size" db:"file_size"`
	LastVerified time.Time `json:"last_verified" db:"last_verified"`
	IsValid      bool      `json:"is_valid" db:"is_valid"`
}

type RecycleBinItem struct {
	ID           int64     `json:"id" db:"id"`
	UserID       int64     `json:"user_id" db:"user_id"`
	FileID       int64     `json:"file_id" db:"file_id"`
	OriginalPath string    `json:"original_path" db:"original_path"`
	OriginalName string    `json:"original_name" db:"original_name"`
	TrashPath    string    `json:"trash_path" db:"trash_path"`
	FileType     string    `json:"file_type" db:"file_type"`
	FileSize     int64     `json:"file_size" db:"file_size"`
	IsDir        int64     `json:"is_dir" db:"is_dir"`
	DeletedAt    time.Time `json:"deleted_at" db:"deleted_at"`
	ExpireAt     time.Time `json:"expire_at" db:"expire_at"`
}

type FileVersion struct {
	ID          int64     `json:"id" db:"id"`
	FileID      int64     `json:"file_id" db:"file_id"`
	VersionNum  int       `json:"version_num" db:"version_num"`
	Size        int64     `json:"size" db:"size"`
	SHA256      string    `json:"sha256" db:"sha256"`
	StoragePath string    `json:"storage_path" db:"storage_path"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
