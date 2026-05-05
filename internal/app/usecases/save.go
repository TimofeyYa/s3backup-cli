// Логика операции сохранения (резервного копирования) директории в S3.
package usecases

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/sypertimka/s3back/internal/archive"
)

// AppVersion — версия приложения, передаётся через ldflags при сборке.
var AppVersion = "0.1.0"

// SaveParams содержит параметры операции сохранения.
type SaveParams struct {
	// SourcePath — путь к директории для резервного копирования
	SourcePath string
	// Bucket — имя S3-бакета
	Bucket string
	// Tag — тег для идентификации резервной копии
	Tag string
	// CompressionLevel — уровень сжатия gzip (1-9)
	CompressionLevel int
	// Progress — функция обратного вызова для отображения прогресса
	Progress ProgressFunc
}

// SaveResult содержит результат операции сохранения.
type SaveResult struct {
	// Key — ключ объекта в S3
	Key string
	// ArchiveSize — размер архива в байтах
	ArchiveSize int64
	// Duration — время выполнения операции
	Duration time.Duration
}

// NormalizeDirName нормализует имя директории для использования в S3-ключе.
// Убирает слэши, заменяет небезопасные символы на подчёркивание.
func NormalizeDirName(name string) string {
	// Убираем слэши с обоих концов
	name = strings.Trim(name, "/\\")
	// Берём только последний компонент пути
	parts := strings.Split(name, "/")
	if len(parts) > 0 {
		name = parts[len(parts)-1]
	}

	// Заменяем небезопасные символы на подчёркивание
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	name = re.ReplaceAllString(name, "_")

	if name == "" {
		name = "backup"
	}
	return name
}

// Save выполняет резервное копирование директории в S3-хранилище.
// Создаёт tar.gz архив и загружает его в бакет с метаданными.
func Save(ctx context.Context, storage Storage, params SaveParams) (*SaveResult, error) {
	startTime := time.Now()

	// Проверяем существование исходной директории
	if _, err := os.Stat(params.SourcePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("исходный путь не существует: %s", params.SourcePath)
	}

	// Нормализуем имя директории для ключа S3
	dirName := NormalizeDirName(params.SourcePath)

	// Формируем timestamp для ключа объекта
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")

	// Формируем ключ объекта в S3
	key := fmt.Sprintf("backups/%s/%s__%s.tar.gz", dirName, timestamp, params.Tag)

	// Создаём временный файл для архива
	tmpFile, err := os.CreateTemp("", "s3back-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("не удалось создать временный файл: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.RemoveAll(tmpPath)

	// Определяем уровень сжатия
	level := params.CompressionLevel
	if level < 1 || level > 9 {
		level = 6
	}

	// Создаём архив
	archiveSize, err := archive.Create(params.SourcePath, tmpPath, level)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания архива: %w", err)
	}

	// Открываем архив для загрузки
	f, err := os.Open(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть архив для загрузки: %w", err)
	}
	defer f.Close()

	// Формируем метаданные для объекта S3
	metadata := map[string]string{
		"backup-tag":   params.Tag,
		"source-name":  dirName,
		"created-at":   time.Now().UTC().Format(time.RFC3339),
		"app-version":  AppVersion,
	}

	// Загружаем архив в S3
	if err := storage.Upload(ctx, params.Bucket, key, f, archiveSize, metadata); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к S3: %w", err)
	}

	return &SaveResult{
		Key:         key,
		ArchiveSize: archiveSize,
		Duration:    time.Since(startTime),
	}, nil
}
