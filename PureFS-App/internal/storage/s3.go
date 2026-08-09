package storage

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Storage struct {
	client *s3.Client
	bucket string
	prefix string
}

type S3Config struct {
	Endpoint  string
	Region    string
	Bucket    string
	AccessKey string
	SecretKey string
	Secure    bool
}

func NewS3Storage(cfg S3Config) (*S3Storage, error) {
	endpoint := cfg.Endpoint
	if !strings.HasPrefix(endpoint, "http") {
		scheme := "https"
		if !cfg.Secure {
			scheme = "http"
		}
		endpoint = fmt.Sprintf("%s://%s", scheme, endpoint)
	}

	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &S3Storage{
		client: client,
		bucket: cfg.Bucket,
	}, nil
}

func (s *S3Storage) s3Key(logicalPath string) string {
	return filepath.ToSlash(strings.TrimPrefix(logicalPath, "/"))
}

func (s *S3Storage) Open(path string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.s3Key(path)),
	})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

type s3WriteCloser struct {
	buf    bytes.Buffer
	client *s3.Client
	bucket string
	key    string
}

func (w *s3WriteCloser) Write(p []byte) (int, error) {
	return w.buf.Write(p)
}

func (w *s3WriteCloser) Close() error {
	data := w.buf.Bytes()

	hash := sha256.Sum256(data)
	log.Printf("[s3] uploading %s (%d bytes, sha256:%s)", w.key, len(data), hex.EncodeToString(hash[:]))

	_, err := w.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &w.bucket,
		Key:    &w.key,
		Body:   bytes.NewReader(data),
	})
	if err != nil {
		return fmt.Errorf("s3 put object: %w", err)
	}

	return nil
}

func (s *S3Storage) Create(path string) (io.WriteCloser, error) {
	return &s3WriteCloser{
		client: s.client,
		bucket: s.bucket,
		key:    s.s3Key(path),
	}, nil
}

func (s *S3Storage) Delete(path string) error {
	_, err := s.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.s3Key(path)),
	})
	return err
}

func (s *S3Storage) Stat(path string) (*FileInfo, error) {
	out, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.s3Key(path)),
	})
	if err != nil {
		return nil, err
	}
	size := int64(0)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}
	return &FileInfo{
		Path:  path,
		Size:  size,
		IsDir: strings.HasSuffix(path, "/"),
	}, nil
}

func (s *S3Storage) List(dir string) ([]*FileInfo, error) {
	prefix := s.s3Key(dir)
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	out, err := s.client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
		Bucket:    &s.bucket,
		Prefix:    &prefix,
		Delimiter: aws.String("/"),
	})
	if err != nil {
		return nil, err
	}

	var result []*FileInfo
	for _, cp := range out.CommonPrefixes {
		name := strings.TrimPrefix(*cp.Prefix, prefix)
		if name == "" {
			continue
		}
		result = append(result, &FileInfo{
			Path:  filepath.Join(dir, name),
			IsDir: true,
		})
	}
	for _, obj := range out.Contents {
		key := strings.TrimPrefix(*obj.Key, prefix)
		if key == "" {
			continue
		}
		objSize := int64(0)
		if obj.Size != nil {
			objSize = *obj.Size
		}
		result = append(result, &FileInfo{
			Path:  filepath.Join(dir, key),
			Size:  objSize,
			IsDir: false,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Path < result[j].Path
	})
	return result, nil
}

func (s *S3Storage) Mkdir(path string) error {
	key := s.s3Key(path)
	if !strings.HasSuffix(key, "/") {
		key += "/"
	}
	_, err := s.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Storage) Rename(oldPath, newPath string) error {
	srcKey := s.s3Key(oldPath)
	dstKey := s.s3Key(newPath)

	_, err := s.client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     &s.bucket,
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, srcKey)),
		Key:        aws.String(dstKey),
	})
	if err != nil {
		return err
	}
	return s.Delete(oldPath)
}

func (s *S3Storage) Copy(srcPath, dstPath string) error {
	_, err := s.client.CopyObject(context.TODO(), &s3.CopyObjectInput{
		Bucket:     &s.bucket,
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, s.s3Key(srcPath))),
		Key:        aws.String(s.s3Key(dstPath)),
	})
	return err
}

func (s *S3Storage) Exists(path string) (bool, error) {
	_, err := s.client.HeadObject(context.TODO(), &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    aws.String(s.s3Key(path)),
	})
	if err != nil {
		var nf *types.NoSuchKey
		_ = nf
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "NoSuchKey") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Storage) RealPath(logicalPath string) string {
	return fmt.Sprintf("s3://%s/%s", s.bucket, s.s3Key(logicalPath))
}

var _ Storage = (*S3Storage)(nil)
