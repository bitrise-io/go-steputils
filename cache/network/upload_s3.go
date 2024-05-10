package network

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
)

const numUploadRetries = 3

// S3UploadParams ...
type S3UploadParams struct {
	ArchivePath     string
	ArchiveChecksum string
	ArchiveSize     int64
	CacheKey        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type s3UploadService struct {
	client          *s3.Client
	bucket          string
	archivePath     string
	archiveChecksum string
	archiveSize     int64
}

// UploadToS3 a cache archive and associate it with the provided cache key
// If there is no match for any of the keys, the error is ErrCacheNotFound.
func UploadToS3(ctx context.Context, params S3UploadParams, logger log.Logger) error {
	validatedKey, err := validateKey(params.CacheKey, logger)
	if err != nil {
		return fmt.Errorf("validate key: %w", err)
	}

	if params.Bucket == "" {
		return fmt.Errorf("Bucket must not be empty")
	}

	if params.ArchivePath == "" {
		return fmt.Errorf("ArchivePath must not be empty")
	}

	if params.ArchiveSize == 0 {
		return fmt.Errorf("ArchiveSize must not be empty")
	}

	cfg, err := loadAWSCredentials(
		ctx,
		params.Region,
		params.AccessKeyID,
		params.SecretAccessKey,
		logger,
	)
	if err != nil {
		return fmt.Errorf("load aws credentials: %w", err)
	}

	client := s3.NewFromConfig(*cfg)
	service := &s3UploadService{
		client:          client,
		bucket:          params.Bucket,
		archivePath:     params.ArchivePath,
		archiveSize:     params.ArchiveSize,
		archiveChecksum: params.ArchiveChecksum,
	}

	return service.uploadWithS3Client(ctx, validatedKey, logger)
}

// If the object for cache key & checksum exists in bucket -> extend expiration
// If the object for cache key exists in bucket -> upload -> overwrites existing object & expiration
// If the object is not yes present in bucket -> upload
func (service *s3UploadService) uploadWithS3Client(ctx context.Context, cacheKey string, logger log.Logger) error {
	checksum, err := service.headObjectWithRetry(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("validate object: %w", err)
	}

	if checksum == service.archiveChecksum {
		// extend expiration time
		err := service.copyObjectWithRetry(ctx, cacheKey, logger)
		if err != nil {
			return fmt.Errorf("copy object: %w", err)
		}
	}

	// upload oject
	err = service.putObjectWithRetry(ctx, cacheKey)
	if err != nil {
		return fmt.Errorf("upload artifact: %w", err)
	}

	return nil
}

func (service *s3UploadService) headObjectWithRetry(ctx context.Context, cacheKey string) (string, error) {
	var checksum string
	err := retry.Times(numUploadRetries).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		resp, err := service.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(service.bucket),
			Key:    aws.String(cacheKey),
		})
		if err != nil {
			var apiError smithy.APIError
			if errors.As(err, &apiError) {
				switch apiError.(type) {
				case *types.NotFound:
					// continue with upload
					break
				default:
					return fmt.Errorf("validating object: %w", err), false
				}
			}
		}

		if resp != nil && resp.ChecksumSHA256 != nil {
			checksum = *resp.ChecksumSHA256
		}

		return nil, true
	})

	return checksum, err
}

func (service *s3UploadService) copyObjectWithRetry(ctx context.Context, cacheKey string, logger log.Logger) error {
	return retry.Times(numUploadRetries).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		resp, err := service.client.CopyObject(ctx, &s3.CopyObjectInput{
			Bucket:       aws.String(service.bucket),
			Key:          aws.String(cacheKey),
			StorageClass: types.StorageClassStandard,
			CopySource:   aws.String(fmt.Sprintf("%s/%s", service.bucket, cacheKey)),
		})
		if err != nil {
			return fmt.Errorf("extend expiration: %w", err), false
		}
		if resp != nil && resp.Expiration != nil {
			logger.Debugf("New expiration date is %s", *resp.Expiration)
		}
		return nil, true
	})
}

func (service *s3UploadService) putObjectWithRetry(ctx context.Context, cacheKey string) error {
	return retry.Times(numUploadRetries).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		_, err := service.client.PutObject(ctx, &s3.PutObjectInput{
			Bucket:            aws.String(service.bucket),
			Key:               aws.String(cacheKey),
			ChecksumSHA256:    aws.String(service.archiveChecksum),
			ContentType:       aws.String("application/zstd"),
			ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		})
		if err != nil {
			return fmt.Errorf("upload artifact: %w", err), false
		}

		return nil, true
	})
}
