package archive

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// TestExtractWithRootMarker проверяет, что Extract корректно обрабатывает архивы,
// содержащие корневые маркеры "./" и ".", не падая с ошибкой "небезопасный путь".
// Такие маркеры часто добавляются GNU tar и другими утилитами.
func TestExtractWithRootMarker(t *testing.T) {
	// Создаём tar.gz с записью "./" в начале.
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	// Корневой маркер "./" — тип Directory.
	if err := tw.WriteHeader(&tar.Header{
		Name:     "./",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	}); err != nil {
		t.Fatalf("не удалось записать заголовок ./: %v", err)
	}

	// Маркер "." — тоже корневой.
	if err := tw.WriteHeader(&tar.Header{
		Name:     ".",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	}); err != nil {
		t.Fatalf("не удалось записать заголовок .: %v", err)
	}

	// Файл с префиксом "./" — должен распаковаться корректно.
	content := []byte("hello world")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "./hello.txt",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatalf("не удалось записать заголовок ./hello.txt: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("не удалось записать содержимое ./hello.txt: %v", err)
	}

	// Поддиректория с префиксом "./".
	if err := tw.WriteHeader(&tar.Header{
		Name:     "./sub/",
		Typeflag: tar.TypeDir,
		Mode:     0755,
	}); err != nil {
		t.Fatalf("не удалось записать заголовок ./sub/: %v", err)
	}

	// Файл во вложенной директории.
	subContent := []byte("nested")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "./sub/nested.txt",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(subContent)),
	}); err != nil {
		t.Fatalf("не удалось записать заголовок ./sub/nested.txt: %v", err)
	}
	if _, err := tw.Write(subContent); err != nil {
		t.Fatalf("не удалось записать содержимое ./sub/nested.txt: %v", err)
	}

	if err := tw.Close(); err != nil {
		t.Fatalf("ошибка закрытия tar: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("ошибка закрытия gzip: %v", err)
	}

	// Записываем архив во временный файл.
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test.tar.gz")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("не удалось записать архив: %v", err)
	}

	// Распаковываем в новую директорию.
	dstDir := filepath.Join(tmpDir, "dst")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		t.Fatalf("не удалось создать dst: %v", err)
	}

	if err := Extract(archivePath, dstDir, false); err != nil {
		t.Fatalf("Extract не должен падать на корневых маркерах ./, получена ошибка: %v", err)
	}

	// Проверяем, что файлы распакованы корректно.
	got, err := os.ReadFile(filepath.Join(dstDir, "hello.txt"))
	if err != nil {
		t.Fatalf("не удалось прочитать hello.txt: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("содержимое hello.txt = %q, want %q", string(got), "hello world")
	}

	gotNested, err := os.ReadFile(filepath.Join(dstDir, "sub", "nested.txt"))
	if err != nil {
		t.Fatalf("не удалось прочитать sub/nested.txt: %v", err)
	}
	if string(gotNested) != "nested" {
		t.Errorf("содержимое sub/nested.txt = %q, want %q", string(gotNested), "nested")
	}
}

// TestExtractDottyNames проверяет, что файлы с точками в именах распаковываются корректно:
// .DS_Store, v1.0, файл..тест.txt, скрытые файлы с ведущей точкой.
func TestExtractDottyNames(t *testing.T) {
	names := []string{
		".DS_Store",
		"v1.0/file.txt",
		"sub/.DS_Store",
		"файл..тест.txt",
		".hidden",
		"a.b.c.d.txt",
	}

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, name := range names {
		content := []byte("test:" + name)
		if err := tw.WriteHeader(&tar.Header{
			Name:     name,
			Typeflag: tar.TypeReg,
			Mode:     0644,
			Size:     int64(len(content)),
		}); err != nil {
			t.Fatalf("не удалось записать заголовок %q: %v", name, err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatalf("не удалось записать содержимое %q: %v", name, err)
		}
	}
	tw.Close()
	gw.Close()

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "dotty.tar.gz")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("не удалось записать архив: %v", err)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	os.MkdirAll(dstDir, 0755)

	if err := Extract(archivePath, dstDir, false); err != nil {
		t.Fatalf("Extract не должен падать на файлах с точками, ошибка: %v", err)
	}

	// Проверяем, что все файлы созданы.
	for _, name := range names {
		p := filepath.Join(dstDir, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("файл %q не был распакован: %v", name, err)
		}
	}
}

// TestExtractRejectsTraversal проверяет, что Extract отклоняет пути с ".." (path traversal).
func TestExtractRejectsTraversal(t *testing.T) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("evil")
	if err := tw.WriteHeader(&tar.Header{
		Name:     "../../../etc/passwd",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     int64(len(content)),
	}); err != nil {
		t.Fatalf("не удалось записать заголовок: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("не удалось записать содержимое: %v", err)
	}
	tw.Close()
	gw.Close()

	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "evil.tar.gz")
	if err := os.WriteFile(archivePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("не удалось записать архив: %v", err)
	}

	dstDir := filepath.Join(tmpDir, "dst")
	os.MkdirAll(dstDir, 0755)

	err := Extract(archivePath, dstDir, false)
	if err == nil {
		t.Fatal("Extract должен был вернуть ошибку для пути с ..")
	}
}
