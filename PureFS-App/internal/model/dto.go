package model

import "time"

type FileInfo struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	FileType   FileType  `json:"file_type"`
	MimeType   string    `json:"mime_type,omitempty"`
	Size       int64     `json:"size"`
	SHA256     string    `json:"sha256,omitempty"`
	IsPinned   bool      `json:"is_pinned"`
	IsFavorite bool      `json:"is_favorite"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type FileListQuery struct {
	ParentID  *int64    `json:"parent_id" schema:"parent_id"`
	SortBy    string    `json:"sort_by" schema:"sort_by"`
	SortOrder string    `json:"sort_order" schema:"sort_order"`
	Offset    int       `json:"offset" schema:"offset"`
	Limit     int       `json:"limit" schema:"limit"`
	FileType  *FileType `json:"file_type" schema:"file_type"`
	Search    string    `json:"search" schema:"search"`
	View      string    `json:"view" schema:"view"`
	UserID    *int64    `json:"-"`
	IsFavorite *bool    `json:"is_favorite,omitempty"`
	IsPinned  *bool     `json:"is_pinned,omitempty"`
}

type CreateFileRequest struct {
	ParentID *int64 `json:"parent_id"`
	Name     string `json:"name"`
	FileType FileType `json:"file_type"`
}

type RenameFileRequest struct {
	Name string `json:"name"`
}

type MoveFileRequest struct {
	TargetParentID int64 `json:"target_parent_id"`
}

type PaginatedResponse struct {
	Items      interface{} `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
	TotalPages int         `json:"total_pages"`
}

type CreateShareRequest struct {
	FileID      int64  `json:"file_id"`
	Password    string `json:"password,omitempty"`
	ExpiresIn   string `json:"expires_in,omitempty"`
	MaxAccesses *int   `json:"max_accesses,omitempty"`
	CanDownload bool   `json:"can_download"`
}

type RegisterUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	TOTPCode string `json:"totp_code,omitempty"`
}

type LoginResponse struct {
	Token        string `json:"token"`
	User         User   `json:"user"`
	TOTPRequired bool   `json:"totp_required"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
