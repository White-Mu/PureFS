package sftp

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/storage"
)

// fileAttrs mirrors the SFTP ATTRS structure.
type fileAttrs struct {
	Flags       uint32
	Size        uint64
	UID         uint32
	GID         uint32
	Permissions uint32
	ATime       uint32
	MTime       uint32
}

// nameEntry is a single directory listing entry.
type nameEntry struct {
	Filename string
	Longname string
	Attrs    fileAttrs
}

// openHandle tracks a file or directory opened by a client.
type openHandle struct {
	path      string
	reader    io.ReadCloser
	writer    io.WriteCloser
	isDir     bool
	dirList   []nameEntry
	dirOffset int
}

// fileSystem is the SFTP filesystem adapter backed by a storage.Storage driver.
// Each instance is bound to a single user and chroots their view to
// /users/{userID}/.
type fileSystem struct {
	store    storage.Storage
	fileRepo *repository.FileRepo
	userID   int64
	rootPath string
	cfg      *config.Config
	mu       sync.Mutex
	handles  map[string]*openHandle // handle -> open file/dir
}

func newFileSystem(store storage.Storage, fileRepo *repository.FileRepo, userID int64, cfg *config.Config) *fileSystem {
	return &fileSystem{
		store:    store,
		fileRepo: fileRepo,
		userID:   userID,
		rootPath: fmt.Sprintf("/users/%d", userID),
		cfg:      cfg,
		handles:  make(map[string]*openHandle),
	}
}

// resolve translates an SFTP-relative path to an absolute storage path,
// enforcing the user's chroot.
func (fs *fileSystem) resolve(path string) (string, error) {
	// Normalize: if the path starts with "/", treat it as relative to root
	clean := filepath.Clean(path)
	if filepath.IsAbs(clean) {
		// Strip leading "/" and join under root
		clean = strings.TrimPrefix(clean, "/")
	}
	resolved := filepath.Join(fs.rootPath, clean)
	// Prevent directory traversal outside the user root
	if !strings.HasPrefix(filepath.Clean(resolved), filepath.Clean(fs.rootPath)) {
		return "", fmt.Errorf("permission denied: path outside user root")
	}
	return resolved, nil
}

// RealPath returns the canonical path for the SFTP realpath command.
func (fs *fileSystem) RealPath(path string) string {
	if path == "." || path == "" {
		return "/"
	}
	resolved, err := fs.resolve(path)
	if err != nil {
		return "/"
	}
	rel := strings.TrimPrefix(resolved, fs.rootPath)
	if rel == "" {
		return "/"
	}
	return "/" + strings.TrimPrefix(rel, "/")
}

// OpenFile handles SSH_FXP_OPEN.
func (fs *fileSystem) OpenFile(handle, path string, flags uint32) error {
	resolved, err := fs.resolve(path)
	if err != nil {
		return err
	}

	read := flags&SSH_FXF_READ != 0
	write := flags&SSH_FXF_WRITE != 0
	creat := flags&SSH_FXF_CREAT != 0
	trunc := flags&SSH_FXF_TRUNC != 0

	exists, _ := fs.store.Exists(resolved)

	fs.mu.Lock()
	defer fs.mu.Unlock()

	h := &openHandle{path: resolved}

	if creat && !exists {
		// Create a new file
		w, err := fs.store.Create(resolved)
		if err != nil {
			return fmt.Errorf("create: %w", err)
		}
		h.writer = w
	} else if trunc && write {
		// Truncate existing file
		w, err := fs.store.Create(resolved)
		if err != nil {
			return fmt.Errorf("truncate: %w", err)
		}
		h.writer = w
	} else if write {
		// Open for writing (append mode not supported through storage interface,
		// so we create a new writer which truncates)
		w, err := fs.store.Create(resolved)
		if err != nil {
			return fmt.Errorf("open for write: %w", err)
		}
		h.writer = w
	}

	if read {
		r, err := fs.store.Open(resolved)
		if err != nil {
			// Clean up writer if we opened one
			if h.writer != nil {
				h.writer.Close()
			}
			return fmt.Errorf("open for read: %w", err)
		}
		h.reader = r
	}

	if !creat && !read && !write && !trunc {
		// Just stat the file
	} else if !exists && !creat {
		return fmt.Errorf("file not found")
	}

	fs.handles[handle] = h
	return nil
}

// CloseFile handles SSH_FXP_CLOSE.
func (fs *fileSystem) CloseFile(handle string) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	h, ok := fs.handles[handle]
	if !ok {
		return fmt.Errorf("invalid handle")
	}
	if h.reader != nil {
		h.reader.Close()
	}
	if h.writer != nil {
		h.writer.Close()
	}
	delete(fs.handles, handle)
	return nil
}

// ReadFile handles SSH_FXP_READ.
func (fs *fileSystem) ReadFile(handle string, offset int64, length int) ([]byte, bool, error) {
	fs.mu.Lock()
	h, ok := fs.handles[handle]
	fs.mu.Unlock()
	if !ok {
		return nil, false, fmt.Errorf("invalid handle")
	}
	if h.reader == nil {
		return nil, false, fmt.Errorf("not opened for reading")
	}

	// storage.Storage.Open returns an io.ReadCloser. We need random-access reads.
	// For simplicity, we re-open the file and seek.
	// In production, a proper file descriptor with Seek would be used.
	r, err := fs.store.Open(h.path)
	if err != nil {
		return nil, false, fmt.Errorf("reopen for read: %w", err)
	}
	defer r.Close()

	// Try to seek if the reader supports it
	if seeker, ok := r.(io.ReadSeeker); ok {
		if _, err := seeker.Seek(offset, io.SeekStart); err != nil {
			return nil, false, fmt.Errorf("seek: %w", err)
		}
	} else {
		// If no seek support, read and discard up to offset
		_, err := io.CopyN(io.Discard, r, offset)
		if err != nil {
			return nil, false, fmt.Errorf("skip: %w", err)
		}
	}

	data := make([]byte, length)
	n, err := io.ReadFull(r, data)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, false, fmt.Errorf("read: %w", err)
	}
	eof := n < length
	return data[:n], eof, nil
}

// WriteFile handles SSH_FXP_WRITE.
func (fs *fileSystem) WriteFile(handle string, offset int64, data []byte) error {
	fs.mu.Lock()
	h, ok := fs.handles[handle]
	fs.mu.Unlock()
	if !ok {
		return fmt.Errorf("invalid handle")
	}
	if h.writer == nil {
		return fmt.Errorf("not opened for writing")
	}
	// Offset-based writes are not natively supported via the storage interface.
	// For a proper SFTP server this would use file descriptors.
	// Simple implementation: append.
	_, err := h.writer.Write(data)
	return err
}

// Stat handles SSH_FXP_STAT / SSH_FXP_LSTAT.
func (fs *fileSystem) Stat(path string) (fileAttrs, error) {
	resolved, err := fs.resolve(path)
	if err != nil {
		return fileAttrs{}, err
	}
	info, err := fs.store.Stat(resolved)
	if err != nil {
		return fileAttrs{}, fmt.Errorf("stat: %w", err)
	}
	return fileAttrsFromStorageInfo(info), nil
}

// FStat handles SSH_FXP_FSTAT.
func (fs *fileSystem) FStat(handle string) (fileAttrs, error) {
	fs.mu.Lock()
	h, ok := fs.handles[handle]
	fs.mu.Unlock()
	if !ok {
		return fileAttrs{}, fmt.Errorf("invalid handle")
	}
	info, err := fs.store.Stat(h.path)
	if err != nil {
		return fileAttrs{}, fmt.Errorf("fstat: %w", err)
	}
	return fileAttrsFromStorageInfo(info), nil
}

// SetStat handles SSH_FXP_SETSTAT.
func (fs *fileSystem) SetStat(path string, attrs []byte) error {
	// Minimal implementation: most setstat operations (chmod, chown, utimes)
	// are not supported by the storage interface. Silently succeed.
	return nil
}

// OpenDir handles SSH_FXP_OPENDIR.
func (fs *fileSystem) OpenDir(handle, path string) error {
	resolved, err := fs.resolve(path)
	if err != nil {
		return err
	}

	exists, _ := fs.store.Exists(resolved)
	if !exists {
		return fmt.Errorf("directory not found")
	}

	entries, err := fs.store.List(resolved)
	if err != nil {
		return fmt.Errorf("list directory: %w", err)
	}

	var names []nameEntry
	for _, e := range entries {
		name := filepath.Base(e.Path)
		attrs := fileAttrsFromStorageInfo(e)
		var longname string
		if e.IsDir {
			longname = dirLongname(name, attrs)
		} else {
			longname = fileLongname(name, attrs)
		}
		names = append(names, nameEntry{
			Filename: name,
			Longname: longname,
			Attrs:    attrs,
		})
	}

	fs.mu.Lock()
	fs.handles[handle] = &openHandle{
		path:    resolved,
		isDir:   true,
		dirList: names,
	}
	fs.mu.Unlock()
	return nil
}

// ReadDir handles SSH_FXP_READDIR.
func (fs *fileSystem) ReadDir(handle string) ([]nameEntry, error) {
	fs.mu.Lock()
	h, ok := fs.handles[handle]
	if !ok {
		fs.mu.Unlock()
		return nil, fmt.Errorf("invalid handle")
	}
	if !h.isDir {
		fs.mu.Unlock()
		return nil, fmt.Errorf("not a directory")
	}
	if h.dirOffset >= len(h.dirList) {
		fs.mu.Unlock()
		return nil, nil
	}
	// Return remaining entries in one shot
	entries := h.dirList[h.dirOffset:]
	h.dirOffset = len(h.dirList)
	fs.mu.Unlock()
	return entries, nil
}

// Remove handles SSH_FXP_REMOVE.
func (fs *fileSystem) Remove(path string) error {
	resolved, err := fs.resolve(path)
	if err != nil {
		return err
	}
	return fs.store.Delete(resolved)
}

// Mkdir handles SSH_FXP_MKDIR.
func (fs *fileSystem) Mkdir(path string) error {
	resolved, err := fs.resolve(path)
	if err != nil {
		return err
	}
	return fs.store.Mkdir(resolved)
}

// Rmdir handles SSH_FXP_RMDIR.
func (fs *fileSystem) Rmdir(path string) error {
	resolved, err := fs.resolve(path)
	if err != nil {
		return err
	}
	return fs.store.Delete(resolved)
}

// Rename handles SSH_FXP_RENAME.
func (fs *fileSystem) Rename(oldPath, newPath string) error {
	old, err := fs.resolve(oldPath)
	if err != nil {
		return err
	}
	newP, err := fs.resolve(newPath)
	if err != nil {
		return err
	}
	return fs.store.Rename(old, newP)
}

// --- helpers ---

// fileAttrsFromStorageInfo builds SFTP file attributes from a storage.FileInfo.
func fileAttrsFromStorageInfo(info *storage.FileInfo) fileAttrs {
	if info == nil {
		return fileAttrs{
			Flags:       SSH_FILEXFER_ATTR_SIZE | SSH_FILEXFER_ATTR_PERMISSIONS,
			Permissions: S_IFREG | 0644,
		}
	}

	attrs := fileAttrs{
		Flags:       SSH_FILEXFER_ATTR_SIZE | SSH_FILEXFER_ATTR_PERMISSIONS,
		Size:        uint64(info.Size),
		UID:         1000,
		GID:         1000,
		ATime:       0,
		MTime:       0,
	}

	if info.IsDir {
		attrs.Permissions = S_IFDIR | 0755
	} else {
		attrs.Permissions = S_IFREG | 0644
	}

	return attrs
}

// dirLongname produces an "ls -l" style long name for a directory.
func dirLongname(name string, attrs fileAttrs) string {
	mode := os.FileMode(attrs.Permissions)
	// drwxr-xr-x  1 user  group  0 Jan 01 00:00 name
	return fmt.Sprintf("%s %4d %-8d %-8d %8d %s %s",
		modeString(mode, true),
		1,
		attrs.UID,
		attrs.GID,
		attrs.Size,
		time.Now().Format("Jan 02 15:04"),
		name,
	)
}

// fileLongname produces an "ls -l" style long name for a file.
func fileLongname(name string, attrs fileAttrs) string {
	mode := os.FileMode(attrs.Permissions)
	// -rw-r--r--  1 user  group  1234 Jan 01 00:00 name
	return fmt.Sprintf("%s %4d %-8d %-8d %8d %s %s",
		modeString(mode, false),
		1,
		attrs.UID,
		attrs.GID,
		attrs.Size,
		time.Now().Format("Jan 02 15:04"),
		name,
	)
}

// modeString returns a Unix "ls -l" style mode string.
func modeString(m os.FileMode, isDir bool) string {
	var buf [10]byte
	if isDir {
		buf[0] = 'd'
	} else {
		buf[0] = '-'
	}
	// User
	if m&0400 != 0 {
		buf[1] = 'r'
	} else {
		buf[1] = '-'
	}
	if m&0200 != 0 {
		buf[2] = 'w'
	} else {
		buf[2] = '-'
	}
	if m&0100 != 0 {
		buf[3] = 'x'
	} else {
		buf[3] = '-'
	}
	// Group
	if m&0040 != 0 {
		buf[4] = 'r'
	} else {
		buf[4] = '-'
	}
	if m&0020 != 0 {
		buf[5] = 'w'
	} else {
		buf[5] = '-'
	}
	if m&0010 != 0 {
		buf[6] = 'x'
	} else {
		buf[6] = '-'
	}
	// Other
	if m&0004 != 0 {
		buf[7] = 'r'
	} else {
		buf[7] = '-'
	}
	if m&0002 != 0 {
		buf[8] = 'w'
	} else {
		buf[8] = '-'
	}
	if m&0001 != 0 {
		buf[9] = 'x'
	} else {
		buf[9] = '-'
	}
	return string(buf[:])
}

// encodeAttrs encodes file attributes into SFTP binary wire format.
func encodeAttrs(attrs fileAttrs) []byte {
	// flags (uint32) + fields
	var buf []byte
	buf = binary.BigEndian.AppendUint32(buf, attrs.Flags)
	if attrs.Flags&SSH_FILEXFER_ATTR_SIZE != 0 {
		buf = binary.BigEndian.AppendUint64(buf, attrs.Size)
	}
	if attrs.Flags&SSH_FILEXFER_ATTR_PERMISSIONS != 0 {
		buf = binary.BigEndian.AppendUint32(buf, attrs.Permissions)
	}
	return buf
}
