// Логика операции скачивания архива из S3.
package usecases

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/sypertimka/s3back/internal/archive"
)

// DownloadParams содержит параметры операции скачивания.
type DownloadParams struct {
	// Bucket — имя S3-бакета
	Bucket string
	// Tag — тег для поиска архива (опционально; если пустой — берётся самый свежий)
	Tag string
	// DestPath — путь для распаковки архива
	DestPath string
	// Force — перезаписывать существующие файлы
	Force bool
	// DryRun — только показать, что будет сделано, без реального выполнения
	DryRun bool
	// Progress — функция обратного вызова для отображения прогресса
	Progress ProgressFunc
}

// DownloadResult содержит результат операции скачивания.
type DownloadResult struct {
	// Key — ключ скачанного объекта
	Key string
	// DestPath — путь, куда распакован архив
	DestPath string
	// Size — размер скачанного архива в байтах
	Size int64
}

// keyPatternWithSource — формат с поддиректорией source: backups/<source>/<timestamp>__<tag>.tar.gz
var keyPatternWithSource = regexp.MustCompile(`^backups/[^/]+/(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z)__([^/]+)\.tar\.gz$`)

// keyPatternFlat — формат без поддиректории: backups/<timestamp>__<tag>.tar.gz
// Поддерживает вариант с двойными подчёркиваниями (__) и тройными (___),
// которые появляются, если source был пустым в старых версиях (обратная совместимость).
var keyPatternFlat = regexp.MustCompile(`^backups/(\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}Z)_+([^/]+)\.tar\.gz$`)

// ParseKey разбирает ключ S3-объекта и извлекает timestamp и тег.
// Поддерживает два формата:
//   - backups/<source>/<timestamp>__<tag>.tar.gz — основной формат
//   - backups/<timestamp>__<tag>.tar.gz — fallback для архивов, залитых раньше с пустым source
// Возвращает timestamp, tag и флаг успешного разбора.
func ParseKey(key string) (timestamp, tag string, ok bool) {
	if m := keyPatternWithSource.FindStringSubmatch(key); m != nil {
		return m[1], m[2], true
	}
	if m := keyPatternFlat.FindStringSubmatch(key); m != nil {
		return m[1], m[2], true
	}
	return "", "", false
}

// Download скачивает архив из S3 и распаковывает его в указанную директорию.
// Если тег не указан, выбирает самый свежий архив по timestamp из имени ключа.
func Download(ctx context.Context, storage Storage, params DownloadParams) (*DownloadResult, error) {
	// Получаем список объектов в бакете с префиксом backups/
	objects, err := storage.List(ctx, params.Bucket, "backups/")
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к S3: %w", err)
	}

	if len(objects) == 0 {
		if params.Tag != "" {
			return nil, fmt.Errorf("архив с тегом %s не найден в bucket %s", params.Tag, params.Bucket)
		}
		return nil, fmt.Errorf("в bucket %s нет архивов", params.Bucket)
	}

	// Фильтруем объекты по тегу (если задан)
	type candidate struct {
		key       string
		timestamp string
	}
	var candidates []candidate

	for _, obj := range objects {
		ts, tag, ok := ParseKey(obj.Key)
		if !ok {
			continue
		}
		if params.Tag != "" && tag != params.Tag {
			continue
		}
		candidates = append(candidates, candidate{key: obj.Key, timestamp: ts})
	}

	if len(candidates) == 0 {
		if params.Tag != "" {
			return nil, fmt.Errorf("архив с тегом %s не найден в bucket %s", params.Tag, params.Bucket)
		}
		return nil, fmt.Errorf("в bucket %s нет подходящих архивов", params.Bucket)
	}

	// Сортируем по timestamp (лексикографически) — формат обеспечивает корректный порядок
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].timestamp > candidates[j].timestamp // убывающий порядок
	})

	// Выбираем самый свежий архив
	selectedKey := candidates[0].key

	// Если dry-run — только выводим информацию
	if params.DryRun {
		fmt.Printf("[dry-run] Будет скачан объект: %s\n", selectedKey)
		fmt.Printf("[dry-run] Назначение: %s\n", params.DestPath)
		return &DownloadResult{Key: selectedKey, DestPath: params.DestPath}, nil
	}

	// Определяем путь назначения
	destPath := params.DestPath
	if destPath == "" {
		destPath = "."
	}

	// Создаём директорию назначения, если не существует
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return nil, fmt.Errorf("не удалось создать директорию назначения %s: %w", destPath, err)
	}

	// Создаём временный файл для скачивания архива
	tmpFile, err := os.CreateTemp("", "s3back-download-*.tar.gz")
	if err != nil {
		return nil, fmt.Errorf("не удалось создать временный файл: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.RemoveAll(tmpPath)

	// Открываем файл для записи скачанного архива
	outFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return nil, fmt.Errorf("не удалось открыть временный файл для записи: %w", err)
	}

	// Скачиваем объект из S3
	if err := storage.Download(ctx, params.Bucket, selectedKey, outFile); err != nil {
		outFile.Close()
		return nil, fmt.Errorf("не удалось подключиться к S3: %w", err)
	}
	outFile.Close()

	// Получаем размер скачанного файла
	archiveInfo, err := os.Stat(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось получить размер архива: %w", err)
	}

	// Распаковываем архив
	if err := archive.Extract(tmpPath, destPath, params.Force); err != nil {
		// Проверяем, является ли ошибка конфликтом файлов
		if strings.Contains(err.Error(), "уже существует") {
			return nil, err
		}
		return nil, fmt.Errorf("ошибка распаковки архива: %w", err)
	}

	return &DownloadResult{
		Key:      selectedKey,
		DestPath: destPath,
		Size:     archiveInfo.Size(),
	}, nil
}
