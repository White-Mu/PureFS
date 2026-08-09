package webdav

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/middleware"
	"github.com/purefs/purefs/internal/model"
	"github.com/purefs/purefs/internal/repository"
	"github.com/purefs/purefs/internal/storage"
	"golang.org/x/net/webdav"
)

type webdavFileSystem struct {
	fileRepo *repository.FileRepo
	store    storage.Storage
	cfg      *config.Config
}

func (fs *webdavFileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	userID := middleware.GetUserID(ctx)
	// Ensure path starts with user's root
	name = fs.userPath(userID, name)
	if err := fs.store.Mkdir(name); err != nil {
		return err
	}
	// Sync with files database table
	parentPath := path.Dir(name)
	dirFile := &model.File{
		UserID:   userID,
		Name:     path.Base(name),
		Path:     name,
		RealPath: fs.store.RealPath(name),
		FileType: model.FileTypeDir,
	}
	// Try to find parent directory for parent_id
	if parentFile, err := fs.fileRepo.GetByUserAndPath(userID, parentPath); err == nil {
		dirFile.ParentID = &parentFile.ID
	}
	_ = fs.fileRepo.Create(dirFile) // best-effort: don't fail the operation if DB insert fails
	return nil
}

func (fs *webdavFileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	userID := middleware.GetUserID(ctx)
	name = fs.userPath(userID, name)

	if flag&os.O_CREATE != 0 {
		if flag&os.O_TRUNC != 0 {
			// Creating new file (O_CREATE|O_TRUNC) — sync DB on close
			w, err := fs.store.Create(name)
			if err != nil {
				return nil, err
			}
			wf := &webdavFile{
				name:   path.Base(name),
				writer: w,
			}
			wf.syncDB = func() {
				parentPath := path.Dir(name)
				f := &model.File{
					UserID:   userID,
					Name:     path.Base(name),
					Path:     name,
					RealPath: fs.store.RealPath(name),
					FileType: model.FileTypeFile,
				}
				if parentFile, err := fs.fileRepo.GetByUserAndPath(userID, parentPath); err == nil {
					f.ParentID = &parentFile.ID
				}
				_ = fs.fileRepo.Create(f) // best-effort
			}
			return wf, nil
		}

		// O_RDWR or O_CREATE alone on existing file — just open for writing, no DB sync
		w, err := fs.store.Create(name)
		if err != nil {
			return nil, err
		}
		return &webdavFile{
			name:   path.Base(name),
			writer: w,
		}, nil
	}

	// Reading
	reader, err := fs.store.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := fs.store.Stat(name)
	if err != nil {
		reader.Close()
		return nil, err
	}

	return &webdavFile{
		name:   path.Base(name),
		reader: reader,
		size:   info.Size,
		isDir:  info.IsDir,
	}, nil
}

func (fs *webdavFileSystem) RemoveAll(ctx context.Context, name string) error {
	userID := middleware.GetUserID(ctx)
	name = fs.userPath(userID, name)
	// Sync with files database: delete the DB record first (best-effort)
	_ = fs.fileRepo.DeleteByUserAndPath(userID, name)
	return fs.store.Delete(name)
}

func (fs *webdavFileSystem) Rename(ctx context.Context, oldName, newName string) error {
	userID := middleware.GetUserID(ctx)
	oldName = fs.userPath(userID, oldName)
	newName = fs.userPath(userID, newName)
	if err := fs.store.Rename(oldName, newName); err != nil {
		return err
	}
	// Sync with files database (best-effort)
	if f, err := fs.fileRepo.GetByUserAndPath(userID, oldName); err == nil {
		_ = fs.fileRepo.Rename(f.ID, path.Base(newName), newName)
	}
	return nil
}

func (fs *webdavFileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	userID := middleware.GetUserID(ctx)
	name = fs.userPath(userID, name)

	info, err := fs.store.Stat(name)
	if err != nil {
		return nil, err
	}

	return &webdavFileInfo{
		name:  path.Base(name),
		size:  info.Size,
		isDir: info.IsDir,
	}, nil
}

func (fs *webdavFileSystem) userPath(userID int64, webdavPath string) string {
	clean := strings.TrimPrefix(webdavPath, "/")
	clean = strings.TrimPrefix(clean, "webdav/")
	return path.Join("/users", fmt.Sprintf("%d", userID), path.Join("/", clean))
}

func Handler(fileRepo *repository.FileRepo, store storage.Storage, cfg *config.Config) http.HandlerFunc {
	dav := &webdav.Handler{
		Prefix:     "/webdav",
		FileSystem: &webdavFileSystem{fileRepo: fileRepo, store: store, cfg: cfg},
		LockSystem: webdav.NewMemLS(),
		Logger: func(r *http.Request, err error) {
			if err != nil {
				println("WebDAV:", r.Method, r.URL.Path, err.Error())
			}
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		dav.ServeHTTP(w, r)
	}
}

type webdavFile struct {
	name   string
	reader io.ReadCloser
	writer io.WriteCloser
	size   int64
	isDir  bool
	offset int64
	syncDB func()
}

func (f *webdavFile) Read(p []byte) (int, error) {
	if f.reader == nil {
		return 0, os.ErrInvalid
	}
	return f.reader.Read(p)
}

func (f *webdavFile) Write(p []byte) (int, error) {
	if f.writer == nil {
		return 0, os.ErrInvalid
	}
	return f.writer.Write(p)
}

func (f *webdavFile) Close() error {
	var err error
	if f.reader != nil {
		err = f.reader.Close()
	}
	if f.writer != nil {
		err = f.writer.Close()
	}
	if f.syncDB != nil {
		f.syncDB()
	}
	return err
}

func (f *webdavFile) Seek(offset int64, whence int) (int64, error) {
	if seeker, ok := f.reader.(io.Seeker); ok {
		return seeker.Seek(offset, whence)
	}
	return 0, os.ErrInvalid
}

func (f *webdavFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, os.ErrInvalid
}

func (f *webdavFile) Stat() (os.FileInfo, error) {
	return &webdavFileInfo{
		name:  f.name,
		size:  f.size,
		isDir: f.isDir,
	}, nil
}

type webdavFileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (i *webdavFileInfo) Name() string      { return i.name }
func (i *webdavFileInfo) Size() int64        { return i.size }
func (i *webdavFileInfo) Mode() os.FileMode  { return 0644 }
func (i *webdavFileInfo) ModTime() time.Time { return time.Now() }
func (i *webdavFileInfo) IsDir() bool        { return i.isDir }
func (i *webdavFileInfo) Sys() interface{}   { return nil }

var _ fs.File = (*webdavFile)(nil)
