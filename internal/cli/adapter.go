// Адаптер для S3-клиента, реализующий интерфейс usecases.Storage.
package cli

import (
	"context"
	"io"
	"time"

	s3client "github.com/sypertimka/s3back/internal/storage/s3"

	"github.com/sypertimka/s3back/internal/app/usecases"
)

// s3StorageAdapter адаптирует *s3client.Client к интерфейсу usecases.Storage.
type s3StorageAdapter struct {
	client *s3client.Client
}

// Upload загружает объект в S3.
func (a *s3StorageAdapter) Upload(ctx context.Context, bucket, key string, body io.Reader, size int64, metadata map[string]string) error {
	return a.client.Upload(ctx, bucket, key, body, size, metadata)
}

// Download скачивает объект из S3.
func (a *s3StorageAdapter) Download(ctx context.Context, bucket, key string, w io.WriterAt) error {
	return a.client.Download(ctx, bucket, key, w)
}

// List возвращает список объектов с префиксом из S3.
func (a *s3StorageAdapter) List(ctx context.Context, bucket, prefix string) ([]usecases.Object, error) {
	objects, err := a.client.List(ctx, bucket, prefix)
	if err != nil {
		return nil, err
	}

	// Конвертируем из типа storage в тип usecases
	result := make([]usecases.Object, len(objects))
	for i, obj := range objects {
		result[i] = usecases.Object{
			Key:          obj.Key,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			Metadata:     obj.Metadata,
		}
	}
	return result, nil
}

// Head возвращает метаданные объекта из S3.
func (a *s3StorageAdapter) Head(ctx context.Context, bucket, key string) (*usecases.Object, error) {
	obj, err := a.client.Head(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	if obj == nil {
		return nil, nil
	}
	return &usecases.Object{
		Key:          obj.Key,
		Size:         obj.Size,
		LastModified: obj.LastModified,
		Metadata:     obj.Metadata,
	}, nil
}

// Обеспечиваем, что s3StorageAdapter реализует usecases.Storage.
// Это заглушка для time.Time чтобы не было неиспользуемого импорта.
var _ = time.Time{}
