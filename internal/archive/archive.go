// Пакет archive реализует создание и распаковку tar.gz архивов.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Create создаёт tar.gz архив из директории srcDir и записывает его в файл dstFile.
// Параметр level задаёт уровень сжатия gzip (1-9).
// Возвращает размер созданного архива в байтах.
// Обрабатывает: обычные файлы, директории (включая пустые), символические ссылки, скрытые файлы.
func Create(srcDir, dstFile string, level int) (int64, error) {
	// Проверяем, что исходная директория существует
	srcInfo, err := os.Lstat(srcDir)
	if err != nil {
		return 0, fmt.Errorf("исходный путь не существует: %s", srcDir)
	}
	if !srcInfo.IsDir() {
		return 0, fmt.Errorf("исходный путь не является директорией: %s", srcDir)
	}

	// Создаём целевой файл архива
	outFile, err := os.Create(dstFile)
	if err != nil {
		return 0, fmt.Errorf("не удалось создать файл архива: %w", err)
	}
	defer outFile.Close()

	// Нормализуем уровень сжатия
	if level < gzip.BestSpeed || level > gzip.BestCompression {
		level = gzip.DefaultCompression
	}

	// Создаём gzip writer с заданным уровнем сжатия
	gw, err := gzip.NewWriterLevel(outFile, level)
	if err != nil {
		return 0, fmt.Errorf("не удалось создать gzip writer: %w", err)
	}
	defer gw.Close()

	// Создаём tar writer
	tw := tar.NewWriter(gw)
	defer tw.Close()

	// Получаем базовое имя директории для формирования путей в архиве
	baseDir := filepath.Base(srcDir)

	// Рекурсивно обходим директорию
	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		// Вычисляем относительный путь внутри архива
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("не удалось вычислить относительный путь: %w", err)
		}

		// Формируем путь в архиве с базовым именем директории
		archivePath := filepath.Join(baseDir, relPath)
		if relPath == "." {
			archivePath = baseDir
		}

		// Используем Lstat для корректной обработки символических ссылок
		linfo, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("не удалось получить информацию о файле %s: %w", path, err)
		}

		// Создаём tar-заголовок на основе информации о файле
		hdr, err := tar.FileInfoHeader(linfo, "")
		if err != nil {
			return fmt.Errorf("не удалось создать заголовок tar для %s: %w", path, err)
		}
		hdr.Name = archivePath

		// Обрабатываем символические ссылки
		if linfo.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("не удалось прочитать символическую ссылку %s: %w", path, err)
			}
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = linkTarget
			// Записываем заголовок без содержимого (ссылки не имеют данных)
			return tw.WriteHeader(hdr)
		}

		// Обрабатываем директории
		if linfo.IsDir() {
			hdr.Typeflag = tar.TypeDir
			if !strings.HasSuffix(hdr.Name, "/") {
				hdr.Name += "/"
			}
			return tw.WriteHeader(hdr)
		}

		// Обрабатываем обычные файлы
		hdr.Typeflag = tar.TypeReg
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("не удалось записать заголовок tar для %s: %w", path, err)
		}

		// Открываем и копируем содержимое файла
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("не удалось открыть файл %s: %w", path, err)
		}
		defer f.Close()

		if _, err := io.Copy(tw, f); err != nil {
			return fmt.Errorf("не удалось записать содержимое файла %s: %w", path, err)
		}

		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("ошибка при создании архива: %w", err)
	}

	// Явно закрываем writer'ы для записи финальных данных
	if err := tw.Close(); err != nil {
		return 0, fmt.Errorf("не удалось завершить tar архив: %w", err)
	}
	if err := gw.Close(); err != nil {
		return 0, fmt.Errorf("не удалось завершить gzip архив: %w", err)
	}
	if err := outFile.Close(); err != nil {
		return 0, fmt.Errorf("не удалось закрыть файл архива: %w", err)
	}

	// Получаем размер созданного файла
	archiveInfo, err := os.Stat(dstFile)
	if err != nil {
		return 0, fmt.Errorf("не удалось получить размер архива: %w", err)
	}

	return archiveInfo.Size(), nil
}

// Extract распаковывает tar.gz архив srcFile в директорию dstDir.
// Если force=false и файл уже существует — возвращает ошибку конфликта.
// Если force=true — перезаписывает существующие файлы.
func Extract(srcFile, dstDir string, force bool) error {
	// Открываем файл архива
	f, err := os.Open(srcFile)
	if err != nil {
		return fmt.Errorf("не удалось открыть архив %s: %w", srcFile, err)
	}
	defer f.Close()

	// Создаём gzip reader
	gr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("не удалось создать gzip reader: %w", err)
	}
	defer gr.Close()

	// Создаём tar reader
	tr := tar.NewReader(gr)

	// Обрабатываем каждую запись в архиве
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // Конец архива
		}
		if err != nil {
			return fmt.Errorf("ошибка чтения архива: %w", err)
		}

		// Формируем целевой путь
		// Сохраняем структуру архива включая верхнеуровневую директорию
		relPath := hdr.Name
		// Убираем ведущие "./" (корневые маркеры в некоторых tar-архивах).
		for strings.HasPrefix(relPath, "./") {
			relPath = relPath[2:]
		}
		// Убираем конечный слэш для директорий при проверке (но не для создания).
		cleanRelPath := strings.TrimSuffix(relPath, "/")
		// Пустые или точечные записи (".", "./") — это корневые маркеры, пропускаем.
		if cleanRelPath == "" || cleanRelPath == "." {
			continue
		}

		// Предотвращаем path traversal: абсолютные пути запрещены.
		if filepath.IsAbs(cleanRelPath) {
			return fmt.Errorf("небезопасный путь в архиве: %s", hdr.Name)
		}
		// Сравниваем очищенный целевой путь с очищенным dstDir.
		// filepath.Join уже вызывает Clean, который схлопывает ".." сегменты.
		// Если после этого путь выходит за пределы dstDir — это попытка traversal.
		// Важно: эта проверка работает корректно для путей с точками в именах
		// (".DS_Store", "v1.0", "файл..тест") — они не выходят за пределы dstDir.
		targetPath := filepath.Join(dstDir, cleanRelPath)
		cleanDst := filepath.Clean(dstDir)
		rel, err := filepath.Rel(cleanDst, targetPath)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("небезопасный путь в архиве: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			// Создаём директорию
			if err := os.MkdirAll(targetPath, hdr.FileInfo().Mode()); err != nil {
				return fmt.Errorf("не удалось создать директорию %s: %w", targetPath, err)
			}

		case tar.TypeSymlink:
			// Создаём символическую ссылку
			if _, err := os.Lstat(targetPath); err == nil {
				if !force {
					return fmt.Errorf("файл уже существует: %s, используйте --force для перезаписи", targetPath)
				}
				if err := os.Remove(targetPath); err != nil {
					return fmt.Errorf("не удалось удалить существующую ссылку %s: %w", targetPath, err)
				}
			}
			// Создаём родительские директории
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("не удалось создать директорию для %s: %w", targetPath, err)
			}
			if err := os.Symlink(hdr.Linkname, targetPath); err != nil {
				return fmt.Errorf("не удалось создать символическую ссылку %s: %w", targetPath, err)
			}

		case tar.TypeReg, tar.TypeRegA:
			// Проверяем конфликты для обычных файлов
			if _, err := os.Stat(targetPath); err == nil {
				if !force {
					return fmt.Errorf("файл уже существует: %s, используйте --force для перезаписи", targetPath)
				}
			}

			// Создаём родительские директории
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("не удалось создать директорию для %s: %w", targetPath, err)
			}

			// Создаём файл
			outFile, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
			if err != nil {
				return fmt.Errorf("не удалось создать файл %s: %w", targetPath, err)
			}

			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("не удалось записать файл %s: %w", targetPath, err)
			}
			outFile.Close()
		}
	}

	return nil
}
