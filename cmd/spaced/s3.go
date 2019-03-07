package main

import (
	"github.com/minio/minio-go"
	"github.com/pkg/errors"
	"net/url"
	"path/filepath"
	"time"
)

type s3Kind int
const (
	s3KindMinio s3Kind = iota
)

type s3Config struct {
	Kind s3Kind `toml:"kind"`

	AccessKey string `toml:"access_key"`
	SecretKey string `toml:"secret_key"`
	Endpoint string `toml:"endpoint"`
	Bucket string `toml:"bucket"`
	Prefix string `toml:"prefix"`
}

type s3Client interface {
	FPutObject(objectName, filePath string) (n int64, err error)
	PresignedGetObject(objectName string, expires time.Duration, reqParams url.Values) (u *url.URL, err error)
}

type minioS3Client struct {
	*minio.Client

	Bucket string
	Prefix string
}

func (c *minioS3Client) FPutObject(objectName, filePath string) (n int64, err error) {
	f := filepath.Join(c.Prefix, objectName)
	return c.Client.FPutObject(c.Bucket, f, filePath, minio.PutObjectOptions{})
}

func (c *minioS3Client) PresignedGetObject(objectName string, expires time.Duration, reqParams url.Values) (u *url.URL, err error) {
	f := filepath.Join(c.Prefix, objectName)
	return c.Client.PresignedGetObject(c.Bucket, f, expires, reqParams)
}

func (c s3Config) GetService() (s3Client, error) {
	if c.Kind != s3KindMinio {
		return nil, errors.Errorf("unsupported S3 kind: %s", c.Kind)
	}
	if len(c.AccessKey) == 0 {
		return nil, errors.New("access-key cannot be blank")
	}
	if len(c.SecretKey) == 0 {
		return nil, errors.New("secret-key cannot be blank")
	}
	if len(c.Endpoint) == 0 {
		return nil, errors.New("endpoint cannot be blank")
	}
	if len(c.Bucket) == 0 {
		return nil, errors.New("bucket cannot be blank")
	}
	cl, err := minio.New(c.Endpoint, c.AccessKey, c.SecretKey, true)
	if err != nil {
		return nil, err
	}
	return &minioS3Client{
		Client: cl,
		Bucket: c.Bucket,
		Prefix: c.Prefix,
	}, nil
}