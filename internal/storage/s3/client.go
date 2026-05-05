// Пакет s3 реализует обёртку над AWS SDK v2 для работы с S3-совместимыми хранилищами.
package s3

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/sypertimka/s3back/internal/config"
)

// Object представляет объект в S3-совместимом хранилище.
type Object struct {
	// Key — ключ объекта в бакете
	Key string
	// Size — размер объекта в байтах
	Size int64
	// LastModified — время последнего изменения
	LastModified time.Time
	// Metadata — метаданные объекта
	Metadata map[string]string
}

// Client — обёртка над клиентом AWS SDK v2 для работы с S3.
type Client struct {
	s3Client   *s3.Client
	uploader   *manager.Uploader
	downloader *manager.Downloader
}

// New создаёт новый S3-клиент на основе конфигурации приложения.
// Поддерживает кастомный endpoint для работы с MinIO, Yandex Object Storage и другими
// S3-совместимыми хранилищами.
func New(cfg *config.Config) (*Client, error) {
	// Создаём статические учётные данные
	creds := credentials.NewStaticCredentialsProvider(
		cfg.AccessKeyID,
		cfg.SecretAccessKey,
		cfg.SessionToken,
	)

	// Формируем базовую конфигурацию AWS
	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(creds),
	)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать AWS конфигурацию: %w", err)
	}

	// Создаём S3-клиент с кастомным endpoint и настройками
	s3Client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
		o.UsePathStyle = cfg.PathStyle
	})

	// Создаём менеджер для multipart загрузки
	uploader := manager.NewUploader(s3Client)

	// Создаём менеджер для multipart скачивания
	downloader := manager.NewDownloader(s3Client)

	return &Client{
		s3Client:   s3Client,
		uploader:   uploader,
		downloader: downloader,
	}, nil
}

// Upload загружает объект в S3-бакет с заданным ключом и метаданными.
// Использует multipart upload для эффективной передачи больших файлов.
func (c *Client) Upload(ctx context.Context, bucket, key string, body io.Reader, size int64, metadata map[string]string) error {
	// Конвертируем метаданные в формат AWS SDK
	awsMetadata := make(map[string]string)
	for k, v := range metadata {
		awsMetadata[k] = v
	}

	_, err := c.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(bucket),
		Key:           aws.String(key),
		Body:          body,
		ContentLength: aws.Int64(size),
		Metadata:      awsMetadata,
	})
	if err != nil {
		return fmt.Errorf("не удалось загрузить объект %s: %w", key, err)
	}

	return nil
}

// Download скачивает объект из S3-бакета и записывает его в writer.
// Использует multipart download для эффективного получения больших файлов.
func (c *Client) Download(ctx context.Context, bucket, key string, w io.WriterAt) error {
	_, err := c.downloader.Download(ctx, w, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("не удалось скачать объект %s: %w", key, err)
	}

	return nil
}

// List возвращает список объектов в бакете с заданным префиксом.
// Поддерживает пагинацию для бакетов с большим количеством объектов.
func (c *Client) List(ctx context.Context, bucket, prefix string) ([]Object, error) {
	var objects []Object
	var continuationToken *string

	for {
		input := &s3.ListObjectsV2Input{
			Bucket: aws.String(bucket),
		}
		if prefix != "" {
			input.Prefix = aws.String(prefix)
		}
		if continuationToken != nil {
			input.ContinuationToken = continuationToken
		}

		resp, err := c.s3Client.ListObjectsV2(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("не удалось получить список объектов: %w", err)
		}

		for _, obj := range resp.Contents {
			o := Object{
				Size:         aws.ToInt64(obj.Size),
				LastModified: aws.ToTime(obj.LastModified),
			}
			if obj.Key != nil {
				o.Key = *obj.Key
			}
			objects = append(objects, o)
		}

		// Проверяем, есть ли ещё страницы
		if !aws.ToBool(resp.IsTruncated) {
			break
		}
		continuationToken = resp.NextContinuationToken
	}

	return objects, nil
}

// Head получает метаданные объекта без загрузки его содержимого.
// Возвращает nil, если объект не найден.
func (c *Client) Head(ctx context.Context, bucket, key string) (*Object, error) {
	resp, err := c.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		// Проверяем, является ли ошибка "объект не найден"
		var notFound *types.NotFound
		if isNotFound(err, &notFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("не удалось получить информацию об объекте %s: %w", key, err)
	}

	obj := &Object{
		Key:          key,
		Size:         aws.ToInt64(resp.ContentLength),
		LastModified: aws.ToTime(resp.LastModified),
		Metadata:     resp.Metadata,
	}

	return obj, nil
}

// isNotFound проверяет, является ли ошибка ошибкой "не найдено".
func isNotFound(err error, target **types.NotFound) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "404") ||
		strings.Contains(errStr, "NoSuchKey") ||
		strings.Contains(errStr, "NotFound")
}
