package storage

import (
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/purefs/purefs/internal/crypto"
)

// EncryptedStorage wraps an inner Storage driver and transparently encrypts
// all file data using AES-256-GCM with per-file Data Encryption Keys.
//
// Each encrypted file has the following on-disk format:
//
//	[1B version][4B kek_version][12B nonce][48B encrypted DEK][AES-256-GCM encrypted data...]
//
// The DEK (Data Encryption Key) is randomly generated per file and encrypted
// with the current KEK (Key Encryption Key) from the KeyManager.
//
// All non-read/write operations (Delete, Stat, List, Mkdir, Rename, Copy, Exists)
// delegate directly to the inner storage driver.
type EncryptedStorage struct {
	inner      Storage
	keyManager *crypto.KeyManager
}

// NewEncryptedStorage creates a new EncryptedStorage wrapping the given storage backend.
func NewEncryptedStorage(inner Storage, keyManager *crypto.KeyManager) *EncryptedStorage {
	return &EncryptedStorage{
		inner:      inner,
		keyManager: keyManager,
	}
}

// KeyManager returns the underlying KeyManager. This is used by the service layer
// to obtain the DEK ciphertext and KEK version for storing in the database.
func (s *EncryptedStorage) KeyManager() *crypto.KeyManager {
	return s.keyManager
}

// Create creates a new encrypted file. It generates a random DEK, encrypts it
// with the current KEK, writes the file header, and returns an EncryptedWriter
// that encrypts all subsequent writes.
//
// Returns the base64-encoded DEK ciphertext and KEK version for storing in the DB,
// plus the writer.
func (s *EncryptedStorage) Create(path string) (io.WriteCloser, error) {
	return s.createEncrypted(path)
}

// createEncrypted generates a DEK, writes the header, and returns an EncryptedWriter.
func (s *EncryptedStorage) createEncrypted(path string) (io.WriteCloser, error) {
	version := s.keyManager.CurrentVersion()
	kek, err := s.keyManager.GetKEK(version)
	if err != nil {
		return nil, fmt.Errorf("get kek: %w", err)
	}

	// Generate random DEK.
	dek, err := generateDEK()
	if err != nil {
		return nil, fmt.Errorf("generate DEK: %w", err)
	}

	// Encrypt DEK with KEK.
	encryptedDEK, err := crypto.EncryptDEK(dek, kek)
	if err != nil {
		return nil, fmt.Errorf("encrypt DEK: %w", err)
	}

	// Build header.
	header := buildHeader(version, encryptedDEK)

	// Open underlying writer.
	inner, err := s.inner.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create inner file: %w", err)
	}

	// Write header.
	if _, err := inner.Write(header); err != nil {
		inner.Close()
		return nil, fmt.Errorf("write header: %w", err)
	}

	// Create encrypting writer.
	ew, err := crypto.NewEncryptedWriter(inner, dek)
	if err != nil {
		inner.Close()
		return nil, fmt.Errorf("new encrypted writer: %w", err)
	}

	return ew, nil
}

// Open opens an encrypted file for reading. It reads and parses the header,
// decrypts the DEK, and returns an EncryptedReader that decrypts the data.
func (s *EncryptedStorage) Open(path string) (io.ReadCloser, error) {
	return s.openEncrypted(path)
}

// openEncrypted reads the header, decrypts the DEK, and returns an EncryptedReader.
func (s *EncryptedStorage) openEncrypted(path string) (io.ReadCloser, error) {
	inner, err := s.inner.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open inner file: %w", err)
	}

	// Read header.
	version, kekVersion, encryptedDEK, err := parseHeader(inner)
	if err != nil {
		inner.Close()
		return nil, fmt.Errorf("parse header: %w", err)
	}
	_ = version // Reserved for future format changes.

	// Get the KEK for the version used when this file was created.
	kek, err := s.keyManager.GetKEK(kekVersion)
	if err != nil {
		inner.Close()
		return nil, fmt.Errorf("get kek v%d: %w", kekVersion, err)
	}

	// Decrypt the DEK.
	dek, err := crypto.DecryptDEK(encryptedDEK, kek)
	if err != nil {
		inner.Close()
		return nil, fmt.Errorf("decrypt DEK: %w", err)
	}

	er, err := crypto.NewEncryptedReader(inner, dek)
	if err != nil {
		inner.Close()
		return nil, fmt.Errorf("new encrypted reader: %w", err)
	}

	return er, nil
}

// Delete removes the file from the underlying storage.
func (s *EncryptedStorage) Delete(path string) error {
	return s.inner.Delete(path)
}

// Stat returns file info from the underlying storage.
func (s *EncryptedStorage) Stat(path string) (*FileInfo, error) {
	fi, err := s.inner.Stat(path)
	if err != nil {
		return nil, err
	}
	// Subtract header size from reported size to get plaintext size.
	if fi != nil && !fi.IsDir {
		fi.Size -= crypto.HeaderSize
		if fi.Size < 0 {
			fi.Size = 0
		}
	}
	return fi, nil
}

// List returns directory entries from the underlying storage.
func (s *EncryptedStorage) List(dir string) ([]*FileInfo, error) {
	return s.inner.List(dir)
}

// Mkdir creates a directory in the underlying storage.
func (s *EncryptedStorage) Mkdir(path string) error {
	return s.inner.Mkdir(path)
}

// Rename moves a file/directory in the underlying storage.
func (s *EncryptedStorage) Rename(oldPath, newPath string) error {
	return s.inner.Rename(oldPath, newPath)
}

// Copy copies a file in the underlying storage.
func (s *EncryptedStorage) Copy(srcPath, dstPath string) error {
	return s.inner.Copy(srcPath, dstPath)
}

// Exists checks if a path exists in the underlying storage.
func (s *EncryptedStorage) Exists(path string) (bool, error) {
	return s.inner.Exists(path)
}

// RealPath returns the actual filesystem path from the underlying storage.
func (s *EncryptedStorage) RealPath(logicalPath string) string {
	return s.inner.RealPath(logicalPath)
}

// ReadHeader reads only the file header and returns the encrypted DEK and KEK version.
// This is useful when the service layer needs to store these values in the database
// after the file has been written through EncryptedWriter.
func ReadHeader(path string, store Storage) (dekCiphertext string, kekVersion int64, err error) {
	rc, err := store.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open file for header: %w", err)
	}
	defer rc.Close()

	version, ver, encryptedDEK, err := parseHeader(rc)
	if err != nil {
		return "", 0, fmt.Errorf("parse header: %w", err)
	}
	_ = version

	return base64.StdEncoding.EncodeToString(encryptedDEK), ver, nil
}

// --- Internal helpers ---

// generateDEK creates a random 32-byte DEK.
func generateDEK() ([]byte, error) {
	return crypto.GenerateDEK()
}

// buildHeader constructs the file header: version + kek_version + encrypted DEK.
func buildHeader(kekVersion int64, encryptedDEK []byte) []byte {
	header := make([]byte, crypto.HeaderSize)
	header[0] = 1 // version
	binary.BigEndian.PutUint32(header[1:5], uint32(kekVersion))
	copy(header[5:], encryptedDEK)
	return header
}

// parseHeader reads and validates the file header from a reader.
// Returns version, kek_version, and encrypted DEK.
func parseHeader(r io.Reader) (version int, kekVersion int64, encryptedDEK []byte, err error) {
	header := make([]byte, crypto.HeaderSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, 0, nil, fmt.Errorf("read header: %w", err)
	}

	version = int(header[0])
	if version != 1 {
		return 0, 0, nil, fmt.Errorf("unsupported file version: %d", version)
	}

	kekVersion = int64(binary.BigEndian.Uint32(header[1:5]))
	encryptedDEK = make([]byte, crypto.DEKCiphertextSize)
	copy(encryptedDEK, header[5:])

	return version, kekVersion, encryptedDEK, nil
}

// Ensure EncryptedStorage implements Storage.
var _ Storage = (*EncryptedStorage)(nil)
