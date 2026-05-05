// Тесты для операции сохранения (резервного копирования).
package usecases

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveUploadsWithCorrectKeyAndMetadata проверяет, что Save вызывает Upload
// с правильным ключом и метаданными.
func TestSaveUploadsWithCorrectKeyAndMetadata(t *testing.T) {
	// Создаём временную директорию с тестовым файлом
	srcDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("тест"), 0644); err != nil {
		t.Fatalf("не удалось создать тестовый файл: %v", err)
	}

	storage := newFakeStorage()
	ctx := context.Background()

	// Выполняем операцию сохранения
	result, err := Save(ctx, storage, SaveParams{
		SourcePath:       srcDir,
		Bucket:           "test-bucket",
		Tag:              "daily",
		CompressionLevel: 6,
	})
	if err != nil {
		t.Fatalf("Save завершился с ошибкой: %v", err)
	}

	// Проверяем результат
	if result == nil {
		t.Fatal("результат Save равен nil")
	}
	if result.ArchiveSize <= 0 {
		t.Errorf("ожидается ArchiveSize > 0, получено %d", result.ArchiveSize)
	}

	// Проверяем, что Upload был вызван
	if len(storage.uploadCalls) != 1 {
		t.Fatalf("ожидается 1 вызов Upload, получено %d", len(storage.uploadCalls))
	}

	call := storage.uploadCalls[0]

	// Проверяем бакет
	if call.bucket != "test-bucket" {
		t.Errorf("бакет: ожидается 'test-bucket', получено %q", call.bucket)
	}

	// Проверяем формат ключа: backups/<source>/<timestamp>__<tag>.tar.gz
	if !strings.HasPrefix(call.key, "backups/") {
		t.Errorf("ключ должен начинаться с 'backups/', получено: %q", call.key)
	}
	if !strings.HasSuffix(call.key, "__daily.tar.gz") {
		t.Errorf("ключ должен заканчиваться на '__daily.tar.gz', получено: %q", call.key)
	}

	// Проверяем метаданные
	if call.metadata["backup-tag"] != "daily" {
		t.Errorf("метаданные backup-tag: ожидается 'daily', получено %q", call.metadata["backup-tag"])
	}
	if call.metadata["source-name"] == "" {
		t.Error("метаданные source-name не должны быть пустыми")
	}
	if call.metadata["created-at"] == "" {
		t.Error("метаданные created-at не должны быть пустыми")
	}
	if call.metadata["app-version"] == "" {
		t.Error("метаданные app-version не должны быть пустыми")
	}

	// Проверяем, что ключ соответствует result.Key
	if result.Key != call.key {
		t.Errorf("result.Key (%q) не совпадает с ключом вызова Upload (%q)", result.Key, call.key)
	}
}

// TestSaveSourceNotExist проверяет, что Save возвращает ошибку,
// если исходная директория не существует.
func TestSaveSourceNotExist(t *testing.T) {
	storage := newFakeStorage()
	ctx := context.Background()

	_, err := Save(ctx, storage, SaveParams{
		SourcePath:       "/несуществующая/директория/12345",
		Bucket:           "test-bucket",
		Tag:              "daily",
		CompressionLevel: 6,
	})

	if err == nil {
		t.Error("ожидалась ошибка при несуществующей директории, получено nil")
	}

	// Проверяем сообщение об ошибке
	if !strings.Contains(err.Error(), "не существует") {
		t.Errorf("сообщение об ошибке должно содержать 'не существует': %s", err.Error())
	}
}

// TestNormalizeDirName проверяет нормализацию имён директорий.
func TestNormalizeDirName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"mydir", "mydir"},
		{"/home/user/backup", "backup"},
		{"my dir", "my_dir"},
		{"тест", "____"},
		{"", "backup"},
		{"my-dir_v2.0", "my-dir_v2.0"},
	}

	for _, tc := range cases {
		result := NormalizeDirName(tc.input)
		if result != tc.expected {
			t.Errorf("NormalizeDirName(%q): ожидается %q, получено %q", tc.input, tc.expected, result)
		}
	}
}
