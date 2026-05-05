// Пакет usecases содержит бизнес-логику приложения s3back.
// Каждая операция (save, download, list) реализована в отдельном файле.
package usecases

import (
	"context"
	"io"
	"time"
)

// Object представляет объект в S3-совместимом хранилище.
type Object struct {
	// Key — ключ объекта
	Key string
	// Size — размер в байтах
	Size int64
	// LastModified — время последнего изменения
	LastModified time.Time
	// Metadata — метаданные объекта
	Metadata map[string]string
}

// Storage — интерфейс для взаимодействия с S3-совместимым хранилищем.
// Используется для инверсии зависимостей (тестируемость через фейковые реализации).
type Storage interface {
	// Upload загружает объект в хранилище
	Upload(ctx context.Context, bucket, key string, body io.Reader, size int64, metadata map[string]string) error
	// Download скачивает объект из хранилища в writer
	Download(ctx context.Context, bucket, key string, w io.WriterAt) error
	// List возвращает список объектов с заданным префиксом
	List(ctx context.Context, bucket, prefix string) ([]Object, error)
	// Head возвращает метаданные объекта (nil если не найден)
	Head(ctx context.Context, bucket, key string) (*Object, error)
}

// ProgressFunc — функция обратного вызова для отображения прогресса операции.
// Принимает количество обработанных байт и общий размер.
type ProgressFunc func(bytesProcessed, totalBytes int64)
