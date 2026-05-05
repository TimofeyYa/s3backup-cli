// Логика операции получения списка архивов из S3.
package usecases

import (
	"context"
	"fmt"
)

// ListParams содержит параметры операции получения списка.
type ListParams struct {
	// Bucket — имя S3-бакета
	Bucket string
	// Source — фильтр по имени источника (опционально)
	Source string
	// DryRun — только показать, что будет сделано
	DryRun bool
}

// ListItem представляет одну запись в списке архивов.
type ListItem struct {
	// Timestamp — метка времени создания архива
	Timestamp string
	// Tag — тег архива
	Tag string
	// Source — имя источника (директории)
	Source string
	// Size — размер в байтах
	Size int64
	// Key — полный ключ объекта в S3
	Key string
}

// List возвращает список архивов в S3-бакете.
// Поддерживает фильтрацию по имени источника.
func List(ctx context.Context, storage Storage, params ListParams) ([]ListItem, error) {
	// Получаем все объекты бакета (архивы хранятся в корне).
	// Фильтрация по source выполняется ниже по метаданным или легаси-ключу.
	objects, err := storage.List(ctx, params.Bucket, "")
	if err != nil {
		return nil, fmt.Errorf("не удалось подключиться к S3: %w", err)
	}

	var items []ListItem
	for _, obj := range objects {
		ts, tag, ok := ParseKey(obj.Key)
		if !ok {
			continue
		}

		// Извлекаем имя источника из ключа (backups/<source>/<timestamp>__<tag>.tar.gz)
		source := extractSource(obj.Key)

		// Фильтруем по источнику (если задан)
		if params.Source != "" && source != params.Source {
			continue
		}

		items = append(items, ListItem{
			Timestamp: ts,
			Tag:       tag,
			Source:    source,
			Size:      obj.Size,
			Key:       obj.Key,
		})
	}

	return items, nil
}

// extractSource извлекает имя источника из ключа S3-объекта (только для легаси-формата).
// Основной формат (в корне бакета) не содержит source в ключе — возвращается "(unknown)".
func extractSource(key string) string {
	const prefix = "backups/"
	if len(key) > len(prefix) && key[:len(prefix)] == prefix {
		rest := key[len(prefix):]
		for i, ch := range rest {
			if ch == '/' {
				return rest[:i]
			}
		}
	}
	return "(unknown)"
}
