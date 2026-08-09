package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Database   DatabaseConfig   `yaml:"database"`
	Storage    StorageConfig    `yaml:"storage"`
	Auth       AuthConfig       `yaml:"auth"`
	Log        LogConfig        `yaml:"log"`
	Integrity  IntegrityConfig  `yaml:"integrity"`
	Trash      TrashConfig      `yaml:"trash"`
	Encryption EncryptionConfig `yaml:"encryption"`
	Search     SearchConfig     `yaml:"search"`
	SFTP       SFTPConfig       `yaml:"sftp"`
	Versioning VersionConfig    `yaml:"versioning"`
}

type ServerConfig struct {
	Host         string        `yaml:"host"`
	Port         int           `yaml:"port"`
	ExternalURL  string        `yaml:"external_url"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	UploadMaxSize int64        `yaml:"upload_max_size"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type StorageConfig struct {
	Driver string `yaml:"driver"` // "local" or "s3"
	Local  LocalStorageConfig  `yaml:"local"`
	S3     S3StorageConfig     `yaml:"s3"`
}

type LocalStorageConfig struct {
	Path string `yaml:"path"`
}

type S3StorageConfig struct {
	Endpoint  string `yaml:"endpoint"`
	Region    string `yaml:"region"`
	Bucket    string `yaml:"bucket"`
	AccessKey string `yaml:"access_key"`
	SecretKey string `yaml:"secret_key"`
	Secure    bool   `yaml:"secure"`
}

type AuthConfig struct {
	JWTSecret     string              `yaml:"jwt_secret"`
	JWTExpiry     time.Duration       `yaml:"jwt_expiry"`
	EncryptionKey string              `yaml:"encryption_key"`
	PasswordPolicy PasswordPolicyConfig `yaml:"password_policy"`
}

type PasswordPolicyConfig struct {
	MinLength      int  `yaml:"min_length"`
	RequireUpper   bool `yaml:"require_upper"`
	RequireLower   bool `yaml:"require_lower"`
	RequireDigit   bool `yaml:"require_digit"`
	RequireSpecial bool `yaml:"require_special"`
}

type LogConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

type IntegrityConfig struct {
	Enabled     bool          `yaml:"enabled"`
	Interval    time.Duration `yaml:"interval"`
}

type TrashConfig struct {
	RetentionDays int `yaml:"retention_days"`
}

type EncryptionConfig struct {
	Enabled               bool   `yaml:"enabled"`
	Key                   string `yaml:"key"`
	RotationIntervalHours int    `yaml:"rotation_interval_hours"`
}

type SearchConfig struct {
	Enabled        bool   `yaml:"enabled"`
	IndexPath      string `yaml:"index_path"`
	MemoryLimitMB  int    `yaml:"memory_limit_mb"`
}

type SFTPConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Port        int    `yaml:"port"`
	HostKeyFile string `yaml:"host_key_file"`
}

type VersionConfig struct {
	MaxVersions int `yaml:"max_versions"`
}

func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host:          "0.0.0.0",
			Port:          8080,
			ExternalURL:   "http://localhost:8080",
			ReadTimeout:   30 * time.Second,
			WriteTimeout:  60 * time.Second,
			UploadMaxSize: 10 << 30,
		},
		Database: DatabaseConfig{
			Driver: "sqlite",
			DSN:    "data/purefs.db",
		},
		Storage: StorageConfig{
			Driver: "local",
			Local: LocalStorageConfig{
				Path: "data/storage",
			},
		},
		Auth: AuthConfig{
			JWTSecret:     "change-me-in-production",
			JWTExpiry:     24 * time.Hour,
			EncryptionKey: "",
			PasswordPolicy: PasswordPolicyConfig{
				MinLength:      8,
				RequireUpper:   true,
				RequireLower:   true,
				RequireDigit:   true,
				RequireSpecial: false,
			},
		},
		Log: LogConfig{
			Level: "info",
		},
		Integrity: IntegrityConfig{
			Enabled:  false,
			Interval: 7 * 24 * time.Hour,
		},
		Trash: TrashConfig{
			RetentionDays: 30,
		},
		Encryption: EncryptionConfig{
			Enabled:               false,
			Key:                   "",
			RotationIntervalHours: 24,
		},
		Search: SearchConfig{
			Enabled:       true,
			IndexPath:     "data/search",
			MemoryLimitMB: 64,
		},
		SFTP: SFTPConfig{
			Enabled:     false,
			Port:        2022,
			HostKeyFile: "data/sftp_host_key",
		},
		Versioning: VersionConfig{
			MaxVersions: 10,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
