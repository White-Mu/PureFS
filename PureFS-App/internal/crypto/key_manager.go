package crypto

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"sync"

	"golang.org/x/crypto/hkdf"
)

// KeyManager derives and caches Key Encryption Keys (KEKs) from a master key.
// Each key rotation increments a version counter. KEKs for each version are
// derived via HKDF-SHA256(master_key, version).
type KeyManager struct {
	mu             sync.RWMutex
	masterKey      []byte
	currentVersion int64
	keks           map[int64][]byte // version -> 32-byte KEK
}

// NewKeyManager creates a new KeyManager from a base64-encoded or raw master key string.
// The master key string is hashed with SHA-256 to produce a consistent 32-byte
// AES-256 key material for HKDF.
func NewKeyManager(masterKey string) (*KeyManager, error) {
	if masterKey == "" {
		return nil, fmt.Errorf("master key must not be empty")
	}
	// Use SHA-256 to normalize any key string to 32 bytes.
	h := sha256.Sum256([]byte(masterKey))
	km := &KeyManager{
		masterKey:      h[:],
		currentVersion: 1,
		keks:           make(map[int64][]byte),
	}
	// Pre-derive the initial KEK.
	if _, err := km.GetKEK(1); err != nil {
		return nil, err
	}
	return km, nil
}

// GetKEK returns the 32-byte AES-256 key encryption key for the given version.
// Derived via HKDF-SHA256(master_key, version_be_bytes).
func (km *KeyManager) GetKEK(version int64) ([]byte, error) {
	km.mu.RLock()
	kek, ok := km.keks[version]
	km.mu.RUnlock()
	if ok {
		return kek, nil
	}

	km.mu.Lock()
	defer km.mu.Unlock()

	// Double-check after acquiring write lock.
	if kek, ok = km.keks[version]; ok {
		return kek, nil
	}

	versionBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(versionBytes, uint64(version))

	reader := hkdf.New(sha256.New, km.masterKey, nil, versionBytes)
	kek = make([]byte, 32)
	if _, err := reader.Read(kek); err != nil {
		return nil, fmt.Errorf("hkdf derive kek v%d: %w", version, err)
	}

	km.keks[version] = kek
	return kek, nil
}

// Rotate increments the key version and derives a new KEK.
// Returns the new version number.
func (km *KeyManager) Rotate() (int64, error) {
	km.mu.Lock()
	defer km.mu.Unlock()

	km.currentVersion++
	newVersion := km.currentVersion

	versionBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(versionBytes, uint64(newVersion))

	reader := hkdf.New(sha256.New, km.masterKey, nil, versionBytes)
	kek := make([]byte, 32)
	if _, err := reader.Read(kek); err != nil {
		return 0, fmt.Errorf("hkdf derive kek v%d: %w", newVersion, err)
	}

	km.keks[newVersion] = kek
	return newVersion, nil
}

// CurrentVersion returns the current key version.
func (km *KeyManager) CurrentVersion() int64 {
	km.mu.RLock()
	defer km.mu.RUnlock()
	return km.currentVersion
}
