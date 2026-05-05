// Тесты для операции скачивания архива.
package usecases

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sypertimka/s3back/internal/archive"
)

// createTestArchive создаёт тестовый tar.gz архив во временной директории.
func createTestArchive(t *testing.T) (srcDir string, archivePath string) {
	t.Helper()

	srcDir = t.TempDir()

	// Создаём тестовый файл
	if err := os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("тестовые данные"), 0644); err != nil {
		t.Fatalf("не удалось создать тестовый файл: %v", err)
	}

	archiveDir := t.TempDir()
	archivePath = filepath.Join(archiveDir, "test.tar.gz")

	_, err := archive.Create(srcDir, archivePath, 6)
	if err != nil {
		t.Fatalf("не удалось создать тестовый архив: %v", err)
	}

	return srcDir, archivePath
}

// TestDownloadSelectsLatestByTimestamp проверяет, что Download без тега
// выбирает самый свежий архив по timestamp.
func TestDownloadSelectsLatestByTimestamp(t *testing.T) {
	_, archivePath := createTestArchive(t)
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("не удалось прочитать тестовый архив: %v", err)
	}

	storage := newFakeStorage()
	bucket := "test-bucket"

	// Добавляем несколько архивов с разными timestamp
	storage.putObject(bucket, "backups/mydir/2024-01-01T10-00-00Z__old.tar.gz", archiveData, nil)
	storage.putObject(bucket, "backups/mydir/2024-06-15T12-30-00Z__newest.tar.gz", archiveData, nil)
	storage.putObject(bucket, "backups/mydir/2024-03-10T08-00-00Z__middle.tar.gz", archiveData, nil)

	ctx := context.Background()
	destDir := t.TempDir()

	result, err := Download(ctx, storage, DownloadParams{
		Bucket:   bucket,
		DestPath: destDir,
		Force:    false,
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("Download завершился с ошибкой: %v", err)
	}

	// Должен выбрать самый новый — 2024-06-15
	if !strings.Contains(result.Key, "2024-06-15") {
		t.Errorf("ожидался архив с timestamp 2024-06-15, выбран: %s", result.Key)
	}
}

// TestDownloadWithTagFilters проверяет, что Download с тегом выбирает правильный архив.
func TestDownloadWithTagFilters(t *testing.T) {
	_, archivePath := createTestArchive(t)
	archiveData, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatalf("не удалось прочитать тестовый архив: %v", err)
	}

	storage := newFakeStorage()
	bucket := "test-bucket"

	storage.putObject(bucket, "backups/mydir/2024-01-01T10-00-00Z__daily.tar.gz", archiveData, nil)
	storage.putObject(bucket, "backups/mydir/2024-06-15T12-30-00Z__weekly.tar.gz", archiveData, nil)

	ctx := context.Background()
	destDir := t.TempDir()

	// Скачиваем с тегом "daily"
	result, err := Download(ctx, storage, DownloadParams{
		Bucket:   bucket,
		Tag:      "daily",
		DestPath: destDir,
		Force:    false,
		DryRun:   false,
	})
	if err != nil {
		t.Fatalf("Download с тегом завершился с ошибкой: %v", err)
	}

	if !strings.Contains(result.Key, "__daily.tar.gz") {
		t.Errorf("ожидался архив с тегом 'daily', выбран: %s", result.Key)
	}
}

// TestDownloadMissingObject проверяет, что Download возвращает понятную ошибку,
// если объект не найден.
func TestDownloadMissingObject(t *testing.T) {
	storage := newFakeStorage()
	ctx := context.Background()
	destDir := t.TempDir()

	// Пытаемся скачать из пустого бакета
	_, err := Download(ctx, storage, DownloadParams{
		Bucket:   "empty-bucket",
		Tag:      "nonexistent",
		DestPath: destDir,
	})

	if err == nil {
		t.Error("ожидалась ошибка при отсутствии объекта, получено nil")
	}

	// Проверяем сообщение об ошибке
	if !strings.Contains(err.Error(), "не найден") && !strings.Contains(err.Error(), "нет архивов") {
		t.Errorf("сообщение об ошибке должно содержать информацию об отсутствии объекта: %s", err.Error())
	}
}

// TestParseKey проверяет парсинг ключей объектов S3.
func TestParseKey(t *testing.T) {
	cases := []struct {
		key       string
		wantTS    string
		wantTag   string
		wantOk    bool
	}{
		{
			"backups/mydir/2024-01-15T10-30-45Z__daily.tar.gz",
			"2024-01-15T10-30-45Z",
			"daily",
			true,
		},
		{
			"backups/my_dir/2024-06-01T00-00-00Z__weekly-backup.tar.gz",
			"2024-06-01T00-00-00Z",
			"weekly-backup",
			true,
		},
		{"invalid-key", "", "", false},
		{"backups/mydir/bad-timestamp__tag.tar.gz", "", "", false},
	}

	for _, tc := range cases {
		ts, tag, ok := ParseKey(tc.key)
		if ok != tc.wantOk {
			t.Errorf("ParseKey(%q): ok ожидается %v, получено %v", tc.key, tc.wantOk, ok)
			continue
		}
		if ok {
			if ts != tc.wantTS {
				t.Errorf("ParseKey(%q): timestamp ожидается %q, получено %q", tc.key, tc.wantTS, ts)
			}
			if tag != tc.wantTag {
				t.Errorf("ParseKey(%q): tag ожидается %q, получено %q", tc.key, tc.wantTag, tag)
			}
		}
	}
}
