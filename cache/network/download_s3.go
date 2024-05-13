package network

import (
	"context"
	"io"
	"os"
	"strings"

	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/bitrise-io/go-utils/retry"
	"github.com/bitrise-io/go-utils/v2/log"
)

// S3DownloadParams ...
type S3DownloadParams struct {
	CacheKeys       []string
	DownloadPath    string
	NumFullRetries  int
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
}

type s3DownloadService struct {
	client         *s3.Client
	bucket         string
	downloadPath   string
	numFullRetries int
}

var errS3KeyNotFound = errors.New("key not found in s3 bucket")

// DownloadFromS3 archive from the provided S3 bucket based on the provided keys in params.
// If there is no match for any of the keys, the error is ErrCacheNotFound.
func DownloadFromS3(ctx context.Context, params S3DownloadParams, logger log.Logger) (string, error) {
	truncatedKeys, err := validateKeys(params.CacheKeys)
	if err != nil {
		return "", fmt.Errorf("validate keys: %w", err)
	}

	if params.Bucket == "" {
		return "", fmt.Errorf("bucket must not be empty")
	}

	cfg, err := loadAWSCredentials(
		ctx,
		params.Region,
		params.AccessKeyID,
		params.SecretAccessKey,
		logger,
	)
	if err != nil {
		return "", fmt.Errorf("load aws credentials: %w", err)
	}

	client := s3.NewFromConfig(*cfg)
	service := &s3DownloadService{
		client:         client,
		bucket:         params.Bucket,
		downloadPath:   params.DownloadPath,
		numFullRetries: params.NumFullRetries,
	}

	return service.downloadWithS3Client(ctx, truncatedKeys, logger)
}

func (service *s3DownloadService) downloadWithS3Client(ctx context.Context, cacheKeys []string, logger log.Logger) (string, error) {
	var firstValidKey string
	err := retry.Times(uint(service.numFullRetries)).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		for _, key := range cacheKeys {
			fileKey := strings.Join([]string{key, "tzst"}, ".")
			keyFound, err := service.firstAvailableKey(ctx, fileKey)
			if err != nil {
				if errors.Is(errS3KeyNotFound, err) {
					logger.Debugf("key %s not found in bucket: %s", key, err)
					continue
				}

				logger.Debugf("validate key %s: %s", key, err)
				return err, false
			}

			firstValidKey = keyFound
			return nil, true
		}
		return ErrCacheNotFound, true
	})
	if err != nil {
		return "", fmt.Errorf("key validation retries failed: %w", err)
	}

	err = retry.Times(uint(service.numFullRetries)).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		if err := service.getObject(ctx, firstValidKey); err != nil {
			return fmt.Errorf("download object: %w", err), false
		}

		return nil, true
	})
	if err != nil {
		return "", fmt.Errorf("all retries failed: %w", err)
	}

	return firstValidKey, nil
}

func (service *s3DownloadService) firstAvailableKey(ctx context.Context, key string) (string, error) {
	_, err := service.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(service.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var apiError smithy.APIError
		if errors.As(err, &apiError) {
			switch apiError.(type) {
			case *types.NotFound:
				return "", errS3KeyNotFound
			default:
				return "", fmt.Errorf("aws api error: %w", err)
			}
		}
		return "", fmt.Errorf("generic aws error: %w", err)
	}

	return key, nil
}

func (service *s3DownloadService) getObject(ctx context.Context, key string) error {
	result, err := service.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(service.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("get object: %w", err)

	}
	defer result.Body.Close() //nolint:errcheck
	file, err := os.Create(service.downloadPath)
	if err != nil {
		return fmt.Errorf("creating file: %w", err)
	}
	defer file.Close() //nolint:errcheck
	body, err := io.ReadAll(result.Body)
	if err != nil {
		return fmt.Errorf("read object content: %w", err)
	}
	_, err = file.Write(body)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func loadAWSCredentials(
	ctx context.Context,
	region string,
	accessKeyID string,
	secretKey string,
	logger log.Logger,
) (*aws.Config, error) {
	if region == "" {
		return nil, fmt.Errorf("region must not be empty")
	}

	opts := []func(*config.LoadOptions) error{
		config.WithRegion(region),
	}

	if accessKeyID != "" && secretKey != "" {
		logger.Debugf("aws credentials provided, using them...")
		opts = append(opts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load config, %v", err)
	}

	return &cfg, nil
}
