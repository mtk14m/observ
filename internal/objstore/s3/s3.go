// Package s3 implements objstore.ObjectStore on any S3-compatible object
// storage: AWS S3, MinIO, Cloudflare R2, Scaleway, OVHcloud, Backblaze…
package s3

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/mtk14n/obsrv/internal/objstore"
)

// Config locates a bucket.
type Config struct {
	// Endpoint is host[:port], for example s3.eu-west-3.amazonaws.com or minio:9000.
	Endpoint string
	Bucket   string
	// Prefix scopes every key under a path of the bucket. Optional.
	Prefix string
	Region string
	// AccessKey and SecretKey are optional: when empty, credentials come from
	// the environment (AWS_* or MINIO_* variables), ~/.aws/credentials or the
	// instance's IAM role.
	AccessKey string
	SecretKey string
	// Insecure uses plain HTTP, for local MinIO.
	Insecure bool
}

// Store is an S3-backed object store.
type Store struct {
	client *minio.Client
	bucket string
	prefix string
}

var _ objstore.ObjectStore = (*Store)(nil)

// New connects to the bucket and checks that it exists.
func New(cfg Config) (*Store, error) {
	creds := credentials.NewChainCredentials([]credentials.Provider{
		&credentials.EnvAWS{}, &credentials.EnvMinio{}, &credentials.FileAWSCredentials{}, &credentials.IAM{},
	})
	if cfg.AccessKey != "" {
		creds = credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, "")
	}
	client, err := minio.New(cfg.Endpoint, &minio.Options{Creds: creds, Secure: !cfg.Insecure, Region: cfg.Region})
	if err != nil {
		return nil, fmt.Errorf("objstore/s3: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ok, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("objstore/s3: check bucket %q: %w", cfg.Bucket, err)
	}
	if !ok {
		return nil, fmt.Errorf("objstore/s3: bucket %q does not exist", cfg.Bucket)
	}
	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	return &Store{client: client, bucket: cfg.Bucket, prefix: prefix}, nil
}

// Put uploads the object. S3 makes the new version visible atomically.
//
// The content is read fully before uploading: obsrv's files are built in
// memory anyway, a known size allows a single PUT request, and a reader
// failing midway never reaches the bucket.
func (s *Store) Put(ctx context.Context, key string, r io.Reader) error {
	if err := objstore.ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("objstore/s3: put %q: read: %w", key, err)
	}
	_, err = s.client.PutObject(ctx, s.bucket, s.prefix+key, bytes.NewReader(data), int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("objstore/s3: put %q: %w", key, err)
	}
	return nil
}

// Get opens the object for reading. The caller must close it.
func (s *Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	if err := objstore.ValidateKey(key); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, s.prefix+key, minio.GetObjectOptions{})
	if err == nil {
		_, err = obj.Stat() // GetObject is lazy: Stat surfaces "not found"
	}
	if err != nil {
		if obj != nil {
			_ = obj.Close()
		}
		if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
			return nil, fmt.Errorf("%w: %q", objstore.ErrNotFound, key)
		}
		return nil, fmt.Errorf("objstore/s3: get %q: %w", key, err)
	}
	return obj, nil
}

// Delete removes the object. Deleting a missing object succeeds.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := objstore.ValidateKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.client.RemoveObject(ctx, s.bucket, s.prefix+key, minio.RemoveObjectOptions{}); err != nil {
		if minio.ToErrorResponse(err).Code == minio.NoSuchKey {
			return nil
		}
		return fmt.Errorf("objstore/s3: delete %q: %w", key, err)
	}
	return nil
}

// List returns the objects whose key starts with prefix, sorted by key.
func (s *Store) List(ctx context.Context, prefix string) ([]objstore.ObjectInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []objstore.ObjectInfo
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.prefix + prefix, Recursive: true}) {
		if obj.Err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("objstore/s3: list %q: %w", prefix, obj.Err)
		}
		out = append(out, objstore.ObjectInfo{
			Key:     strings.TrimPrefix(obj.Key, s.prefix),
			Size:    obj.Size,
			ModTime: obj.LastModified,
		})
	}
	return out, nil
}
