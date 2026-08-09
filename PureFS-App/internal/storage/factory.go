package storage

import (
	"fmt"

	"github.com/purefs/purefs/internal/config"
	"github.com/purefs/purefs/internal/crypto"
)

func NewFromConfig(driver string, localPath string, s3Cfg S3Config) (Storage, error) {
	switch driver {
	case "local":
		return NewLocalStorage(localPath)
	case "s3":
		return NewS3Storage(s3Cfg)
	default:
		return nil, fmt.Errorf("unknown storage driver: %s", driver)
	}
}

// NewFromConfigWithEncryption creates a storage backend from config and optionally
// wraps it with EncryptedStorage if encryption is enabled.
func NewFromConfigWithEncryption(driver string, localPath string, s3Cfg S3Config, encCfg config.EncryptionConfig) (Storage, *crypto.KeyManager, error) {
	base, err := NewFromConfig(driver, localPath, s3Cfg)
	if err != nil {
		return nil, nil, err
	}

	if !encCfg.Enabled {
		return base, nil, nil
	}

	km, err := crypto.NewKeyManager(encCfg.Key)
	if err != nil {
		return nil, nil, fmt.Errorf("create key manager: %w", err)
	}

	wrapped := NewEncryptedStorage(base, km)
	return wrapped, km, nil
}
