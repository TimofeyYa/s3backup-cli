// Тесты для пакета archive.
package archive

import (
	"crypto/sha256"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// createTestDir создаёт временную директорию с набором тестовых файлов и поддиректорией.
func createTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Обычный файл в корне
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("содержимое файла 1"), 0644); err != nil {
		t.Fatalf("не удалось создать file1.txt: %v", err)
	}

	// Скрытый файл (начинается с точки)
	if err := os.WriteFile(filepath.Join(dir, ".hidden"), []byte("скрытый файл"), 0644); err != nil {
		t.Fatalf("не удалось создать .hidden: %v", err)
	}

	// Поддиректория с файлом
	subDir := filepath.Join(dir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("не удалось создать subdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("содержимое файла 2"), 0644); err != nil {
		t.Fatalf("не удалось создать file2.txt: %v", err)
	}

	// Пустая поддиректория
	emptyDir := filepath.Join(dir, "empty")
	if err := os.MkdirAll(emptyDir, 0755); err != nil {
		t.Fatalf("не удалось создать empty dir: %v", err)
	}

	return dir
}

// hashFile вычисляет SHA256 хеш файла.
func hashFile(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("не удалось открыть файл для хеширования %s: %v", path, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		t.Fatalf("не удалось вычислить хеш %s: %v", path, err)
	}
	return h.Sum(nil)
}

// TestCreateExtractRoundTrip проверяет полный цикл: создание архива и его распаковка.
// Сравнивает содержимое исходной и распакованной директорий.
func TestCreateExtractRoundTrip(t *testing.T) {
	srcDir := createTestDir(t)
	archiveFile := filepath.Join(t.TempDir(), "test.tar.gz")
	dstDir := t.TempDir()

	// Создаём архив
	size, err := Create(srcDir, archiveFile, 6)
	if err != nil {
		t.Fatalf("Create завершился с ошибкой: %v", err)
	}
	if size <= 0 {
		t.Errorf("ожидается размер > 0, получено %d", size)
	}

	// Проверяем, что файл архива существует
	archiveInfo, err := os.Stat(archiveFile)
	if err != nil {
		t.Fatalf("файл архива не создан: %v", err)
	}
	if archiveInfo.Size() != size {
		t.Errorf("размер архива не совпадает: ожидается %d, получено %d", size, archiveInfo.Size())
	}

	// Распаковываем архив
	if err := Extract(archiveFile, dstDir, false); err != nil {
		t.Fatalf("Extract завершился с ошибкой: %v", err)
	}

	// Получаем базовое имя исходной директории
	baseName := filepath.Base(srcDir)

	// Проверяем наличие и содержимое файлов
	testCases := []struct {
		relPath  string
		content  string
		isDir    bool
	}{
		{"file1.txt", "содержимое файла 1", false},
		{".hidden", "скрытый файл", false},
		{"subdir/file2.txt", "содержимое файла 2", false},
		{"subdir", "", true},
		{"empty", "", true},
	}

	for _, tc := range testCases {
		dstPath := filepath.Join(dstDir, baseName, tc.relPath)
		info, err := os.Lstat(dstPath)
		if err != nil {
			t.Errorf("файл/директория не найдена в распакованном архиве: %s — %v", tc.relPath, err)
			continue
		}

		if tc.isDir {
			if !info.IsDir() {
				t.Errorf("%s должна быть директорией", tc.relPath)
			}
			continue
		}

		// Проверяем содержимое файла
		data, err := os.ReadFile(dstPath)
		if err != nil {
			t.Errorf("не удалось прочитать %s: %v", tc.relPath, err)
			continue
		}
		if string(data) != tc.content {
			t.Errorf("%s: ожидается %q, получено %q", tc.relPath, tc.content, string(data))
		}
	}
}

// TestSourceNotModified проверяет, что операция Create не изменяет исходные файлы.
func TestSourceNotModified(t *testing.T) {
	srcDir := createTestDir(t)
	archiveFile := filepath.Join(t.TempDir(), "test.tar.gz")

	// Записываем хеши и время изменения исходных файлов до архивации
	type fileState struct {
		hash  []byte
		mtime time.Time
	}
	statesBefore := make(map[string]fileState)

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		statesBefore[path] = fileState{
			hash:  hashFile(t, path),
			mtime: info.ModTime(),
		}
		return nil
	})
	if err != nil {
		t.Fatalf("не удалось обойти исходную директорию: %v", err)
	}

	// Создаём архив
	if _, err := Create(srcDir, archiveFile, 6); err != nil {
		t.Fatalf("Create завершился с ошибкой: %v", err)
	}

	// Проверяем, что исходные файлы не изменились
	for path, before := range statesBefore {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("исходный файл исчез после архивации: %s", path)
			continue
		}
		afterHash := hashFile(t, path)
		// Сравниваем хеши
		for i, b := range before.hash {
			if afterHash[i] != b {
				t.Errorf("хеш файла изменился: %s", path)
				break
			}
		}
		// Проверяем время изменения (с допуском в 1 секунду)
		if info.ModTime().Sub(before.mtime).Abs() > time.Second {
			t.Errorf("время изменения файла изменилось: %s", path)
		}
	}
}

// TestExtractConflict проверяет поведение при конфликте файлов при распаковке.
// Без force должна быть ошибка, с force — перезапись.
func TestExtractConflict(t *testing.T) {
	srcDir := createTestDir(t)
	archiveFile := filepath.Join(t.TempDir(), "test.tar.gz")

	// Создаём архив
	if _, err := Create(srcDir, archiveFile, 6); err != nil {
		t.Fatalf("Create завершился с ошибкой: %v", err)
	}

	// Первое извлечение — должно пройти успешно
	dstDir := t.TempDir()
	if err := Extract(archiveFile, dstDir, false); err != nil {
		t.Fatalf("первый Extract завершился с ошибкой: %v", err)
	}

	// Второе извлечение без force — должна быть ошибка
	err := Extract(archiveFile, dstDir, false)
	if err == nil {
		t.Error("ожидалась ошибка при конфликте без force, получено nil")
	} else {
		// Проверяем, что сообщение содержит ссылку на --force
		if !strings.Contains(err.Error(), "force") {
			t.Errorf("сообщение об ошибке не содержит 'force': %s", err.Error())
		}
	}

	// Второе извлечение с force — должно перезаписать
	if err := Extract(archiveFile, dstDir, true); err != nil {
		t.Errorf("Extract с force завершился с ошибкой: %v", err)
	}

	// Проверяем, что содержимое после force-перезаписи корректно
	baseName := filepath.Base(srcDir)
	data, err := os.ReadFile(filepath.Join(dstDir, baseName, "file1.txt"))
	if err != nil {
		t.Fatalf("не удалось прочитать файл после force-перезаписи: %v", err)
	}
	if string(data) != "содержимое файла 1" {
		t.Errorf("содержимое файла после перезаписи некорректно: %s", string(data))
	}
}
