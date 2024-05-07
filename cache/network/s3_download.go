package network

import (
	"context"
	"io"
	"os"

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

// DownloadFromS3 archive from the provided S3 bucket based on the provided keys in params.
// If there is no match for any of the keys, the error is ErrCacheNotFound.
func DownloadFromS3(ctx context.Context, params S3DownloadParams, logger log.Logger) (string, error) {
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
	return downloadWithS3Client(ctx, client, params, logger)
}

func downloadWithS3Client(ctx context.Context, client *s3.Client, params S3DownloadParams, logger log.Logger) (string, error) {
	var matchedKey string
	var firstValidKey string
	err := retry.Times(uint(params.NumFullRetries)).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		for _, key := range params.CacheKeys {
			_, err := client.HeadObject(ctx, &s3.HeadObjectInput{
				Bucket: &params.Bucket,
				Key:    aws.String(key),
			})
			if err != nil {
				var apiError smithy.APIError
				if errors.As(err, &apiError) {
					switch apiError.(type) {
					case *types.NotFound:
						logger.Debugf("key %s not found in bucket: %s", key, err)
						continue
					default:
						logger.Debugf("validate key %s: %s", key, err)
						return err, false
					}
				}
			}
			firstValidKey = key
			break
		}
		return ErrCacheNotFound, true
	})
	if err != nil {
		return "", fmt.Errorf("key validation retries failed: %w", err)
	}

	err = retry.Times(uint(params.NumFullRetries)).Wait(5 * time.Second).TryWithAbort(func(attempt uint) (error, bool) {
		result, err := client.GetObject(ctx, &s3.GetObjectInput{
			Bucket: aws.String(params.Bucket),
			Key:    aws.String(firstValidKey),
		})
		if err != nil {
			return fmt.Errorf("get object: %w", err), false
		}
		defer result.Body.Close() //nolint:errcheck
		file, err := os.Create(params.DownloadPath)
		if err != nil {
			return fmt.Errorf("creating file: %w", err), true
		}
		defer file.Close() //nolint:errcheck
		body, err := io.ReadAll(result.Body)
		if err != nil {
			return fmt.Errorf("read object content: %w", err), false
		}
		_, err = file.Write(body)
		if err != nil {
			return fmt.Errorf("write file: %w", err), true
		}

		return nil, true
	})
	if err != nil {
		return "", fmt.Errorf("all retries failed: %w", err)
	}

	return matchedKey, nil
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
		logger.Debugf("aws credentials not defined, loading credentials from environemnt...")
		opts = append(opts,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretKey, "")))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load config, %v", err)
	}

	return &cfg, nil
}
