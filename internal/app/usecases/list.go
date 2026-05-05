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
	// Формируем префикс для фильтрации
	prefix := "backups/"
	if params.Source != "" {
		prefix = fmt.Sprintf("backups/%s/", params.Source)
	}

	// Получаем список объектов
	objects, err := storage.List(ctx, params.Bucket, prefix)
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

// extractSource извлекает имя источника из ключа S3-объекта.
// Формат ключа: backups/<source>/<timestamp>__<tag>.tar.gz
func extractSource(key string) string {
	// Убираем префикс "backups/"
	if len(key) < 8 {
		return ""
	}
	rest := key[8:] // после "backups/"
	// Ищем следующий слэш
	for i, ch := range rest {
		if ch == '/' {
			return rest[:i]
		}
	}
	return rest
}
