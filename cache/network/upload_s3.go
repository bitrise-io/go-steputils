package network

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
)

const (
	numUploadRetries = 3

	megabytes = 1024 * 1024

	// 50MB
	multipartChunkSize = 50_000_000

	// The archive checksum is uploaded with the object as metadata.
	// As we can be pretty sure of the integrity of the uploaded archive,
	// so we avoid having to locally pre-calculate the SHA-256 checksum of each (multi)part.
	// Instead we include the full object checksum as metadata, and compare against it at consecutive uploads.
	checksumKey = "full-object-checksum-sha256"
)

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
	awsCacheKey := fmt.Sprintf("%s.%s", cacheKey, "tzst")
	checksum, err := service.findChecksumWithRetry(ctx, awsCacheKey)
	if err != nil {
		return fmt.Errorf("validate object: %w", err)
	}

	if checksum == service.archiveChecksum {
		logger.Debugf("Found archive with the same checksum. Extending expiration time...")
		err := service.copyObjectWithRetry(ctx, awsCacheKey, logger)
		if err != nil {
			return fmt.Errorf("copy object: %w", err)
		}
		return nil
	}

	logger.Debugf("Uploading archive...")
	err = service.putObjectWithRetry(ctx, awsCacheKey)
	if err != nil {
		return fmt.Errorf("upload artifact: %w", err)
	}

	return nil
}

// findChecksumWithRetry tries to find the archive in bucket.
// If the object is present, it returns the saved SHA-256 checksum from metadata.
// If the object isn't present, it returns an empty string.
func (service *s3UploadService) findChecksumWithRetry(ctx context.Context, cacheKey string) (string, error) {
	var checksum string
	err := retry.Times(numUploadRetries).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		response, err := service.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(service.bucket),
			Key:    aws.String(cacheKey),
		})
		if err != nil {
			var apiError smithy.APIError
			if errors.As(err, &apiError) {
				switch apiError.(type) {
				case *types.NotFound:
					// continue with upload
					return nil, true
				default:
					return fmt.Errorf("validating object: %w", err), false
				}
			}
		}

		if response.Metadata != nil {
			if sha256, ok := response.Metadata[checksumKey]; ok {
				checksum = sha256
			}
		}

		return nil, true
	})

	return checksum, err
}

// By copying an S3 object into itself with the same Storage Class, the expiration date gets extended.
// copyObjectWithRetry uses this trick to extend archive expiration.
func (service *s3UploadService) copyObjectWithRetry(ctx context.Context, cacheKey string, logger log.Logger) error {
	if service.archiveSize < 100*megabytes {
		logger.Debugf("Performing simple copy")
		return retry.Times(numUploadRetries).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
			resp, err := service.client.CopyObject(ctx, &s3.CopyObjectInput{
				Bucket:       aws.String(service.bucket),
				Key:          aws.String(cacheKey),
				StorageClass: types.StorageClassStandard,
				CopySource:   aws.String(fmt.Sprintf("%s/%s", service.bucket, cacheKey)),
				Metadata: map[string]string{
					checksumKey: service.archiveChecksum,
				},
			})
			if err != nil {
				return fmt.Errorf("extend expiration: %w", err), false
			}
			if resp != nil && resp.Expiration != nil {
				logger.Debugf("New expiration date is %s", *resp.Expiration)
			}
			return nil, true
		})
	} else {
		// Object bigger than 5GB cannot be copied by CopyObject, MultipartCopy is the way to go
		logger.Debugf("Performing multipart copy")
		return service.copyObjectMultipart(ctx, cacheKey, logger)
	}
}

func (service *s3UploadService) putObjectWithRetry(ctx context.Context, cacheKey string) error {
	return retry.Times(numUploadRetries).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		file, err := os.Open(service.archivePath)
		if err != nil {
			return fmt.Errorf("open archive path: %w", err), true
		}
		defer file.Close() //nolint:errcheck

		uploader := manager.NewUploader(service.client, func(u *manager.Uploader) {
			u.PartSize = multipartChunkSize
			u.Concurrency = runtime.NumCPU()
		})

		_, err = uploader.Upload(ctx, &s3.PutObjectInput{
			Body:              file,
			Bucket:            aws.String(service.bucket),
			Key:               aws.String(cacheKey),
			ContentType:       aws.String("application/zstd"),
			ContentLength:     aws.Int64(service.archiveSize),
			ContentEncoding:   aws.String("zstd"),
			ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
			Metadata: map[string]string{
				checksumKey: service.archiveChecksum,
			},
		})
		if err != nil {
			return fmt.Errorf("upload artifact: %w", err), false
		}

		return nil, true
	})
}

// perform multipart oject copy concurrently
func (service *s3UploadService) copyObjectMultipart(ctx context.Context, cacheKey string, logger log.Logger) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	initUploadInput := s3.CreateMultipartUploadInput{
		Bucket:            aws.String(service.bucket),
		Key:               aws.String(cacheKey),
		ChecksumAlgorithm: types.ChecksumAlgorithmSha256,
		StorageClass:      types.StorageClassStandard,
		Metadata: map[string]string{
			checksumKey: service.archiveChecksum,
		},
	}

	var operationID string
	initUploadResponse, err := service.client.CreateMultipartUpload(ctx, &initUploadInput)
	if err != nil {
		return fmt.Errorf("start multipart copy: %w", err)
	}
	if initUploadResponse != nil && initUploadResponse.UploadId != nil {
		if *initUploadResponse.UploadId == "" {
			return fmt.Errorf("upload ID was empty: %w", err)
		}
		operationID = *initUploadResponse.UploadId
	}

	var wg sync.WaitGroup
	completed := make(chan types.CompletedPart)
	errc := make(chan error)

	completedParts := make([]types.CompletedPart, 0)
	partID := 1
	logger.Debugf("Will copy %d parts", service.archiveSize/multipartChunkSize+1)
	for i := 0; i < int(service.archiveSize); i += multipartChunkSize {
		wg.Add(1)

		go func(wg *sync.WaitGroup, iteration int, partID int) {
			defer wg.Done()
			sourceRange := service.copyObjectSourceRange(iteration)
			multipartInput := &s3.UploadPartCopyInput{
				Bucket:          aws.String(service.bucket),
				CopySource:      aws.String(fmt.Sprintf("%s/%s", service.bucket, cacheKey)),
				CopySourceRange: aws.String(sourceRange),
				Key:             aws.String(cacheKey),
				PartNumber:      aws.Int32(int32(partID)),
				UploadId:        aws.String(operationID),
			}

			multipartUploadResponse, err := service.client.UploadPartCopy(ctx, multipartInput)
			if err != nil {
				logger.Debugf("Aborting multipart copy")
				service.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{ //nolint:errcheck
					UploadId: aws.String(operationID),
				})
				errc <- fmt.Errorf("abort multipart copy operation: %w", err)
			}
			if multipartUploadResponse != nil && multipartUploadResponse.CopyPartResult != nil && multipartUploadResponse.CopyPartResult.ETag != nil {
				etag := strings.Trim(*multipartUploadResponse.CopyPartResult.ETag, "\"")
				completedParts = append(completedParts, types.CompletedPart{
					ETag:           aws.String(etag),
					PartNumber:     aws.Int32(int32(partID)),
					ChecksumSHA256: multipartUploadResponse.CopyPartResult.ChecksumSHA256,
				})
			}
			logger.Debugf("Multipart copy part #%d completed", partID)
		}(&wg, i, partID)
		partID++
	}

	go func() {
		wg.Wait()
		close(completed)
		close(errc)
	}()

	go func(cancel context.CancelFunc) {
		for eCh := range errc {
			if eCh != nil {
				logger.Errorf("multipart copy: %w", err)
				cancel()
			}
		}
	}(cancel)

	for c := range completed {
		completedParts = append(completedParts, c)
	}

	sort.Slice(completedParts, func(i, j int) bool {
		return *completedParts[i].PartNumber < *completedParts[j].PartNumber
	})

	parts := &types.CompletedMultipartUpload{
		Parts: completedParts,
	}
	completeCopyInput := &s3.CompleteMultipartUploadInput{
		Bucket:          aws.String(service.bucket),
		Key:             aws.String(cacheKey),
		UploadId:        aws.String(operationID),
		MultipartUpload: parts,
	}

	completeCopyOutput, err := service.client.CompleteMultipartUpload(ctx, completeCopyInput)
	if err != nil {
		return fmt.Errorf("coplete multipart copy: %w", err)
	}
	if completeCopyOutput != nil && completeCopyOutput.Expiration != nil {
		logger.Debugf("New expiration date is %s", *completeCopyOutput.Expiration)
	}
	return nil
}

func (service *s3UploadService) copyObjectSourceRange(part int) string {
	end := part + multipartChunkSize - 1
	if end > int(service.archiveSize) {
		end = int(service.archiveSize) - 1
	}
	return fmt.Sprintf("bytes=%d-%d", part, end)
}
