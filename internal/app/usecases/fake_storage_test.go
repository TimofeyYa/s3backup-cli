// Фейковая реализация интерфейса Storage для тестирования бизнес-логики без реального S3.
package usecases

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// fakeObject представляет объект в in-memory хранилище.
type fakeObject struct {
	data     []byte
	metadata map[string]string
	size     int64
}

// fakeStorage — in-memory реализация интерфейса Storage для тестов.
type fakeStorage struct {
	mu      sync.RWMutex
	objects map[string]map[string]fakeObject // bucket -> key -> object

	// Перехваченные вызовы Upload для проверки
	uploadCalls []uploadCall
}

// uploadCall хранит параметры вызова Upload.
type uploadCall struct {
	bucket   string
	key      string
	size     int64
	metadata map[string]string
}

// newFakeStorage создаёт новое пустое in-memory хранилище.
func newFakeStorage() *fakeStorage {
	return &fakeStorage{
		objects: make(map[string]map[string]fakeObject),
	}
}

// putObject добавляет объект напрямую (для настройки тестов).
func (s *fakeStorage) putObject(bucket, key string, data []byte, metadata map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objects[bucket] == nil {
		s.objects[bucket] = make(map[string]fakeObject)
	}
	s.objects[bucket][key] = fakeObject{
		data:     data,
		metadata: metadata,
		size:     int64(len(data)),
	}
}

// Upload реализует загрузку объекта в in-memory хранилище.
func (s *fakeStorage) Upload(ctx context.Context, bucket, key string, body io.Reader, size int64, metadata map[string]string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return fmt.Errorf("не удалось прочитать данные: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.objects[bucket] == nil {
		s.objects[bucket] = make(map[string]fakeObject)
	}
	s.objects[bucket][key] = fakeObject{
		data:     data,
		metadata: metadata,
		size:     size,
	}

	// Записываем вызов для проверки в тестах
	metaCopy := make(map[string]string)
	for k, v := range metadata {
		metaCopy[k] = v
	}
	s.uploadCalls = append(s.uploadCalls, uploadCall{
		bucket:   bucket,
		key:      key,
		size:     size,
		metadata: metaCopy,
	})

	return nil
}

// Download реализует скачивание объекта из in-memory хранилища.
func (s *fakeStorage) Download(ctx context.Context, bucket, key string, w io.WriterAt) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bucketObjs, ok := s.objects[bucket]
	if !ok {
		return fmt.Errorf("бакет %s не найден", bucket)
	}

	obj, ok := bucketObjs[key]
	if !ok {
		return fmt.Errorf("объект %s не найден в бакете %s", key, bucket)
	}

	_, err := w.WriteAt(obj.data, 0)
	return err
}

// List реализует получение списка объектов из in-memory хранилища.
func (s *fakeStorage) List(ctx context.Context, bucket, prefix string) ([]Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bucketObjs, ok := s.objects[bucket]
	if !ok {
		return nil, nil
	}

	var result []Object
	for key, obj := range bucketObjs {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			result = append(result, Object{
				Key:      key,
				Size:     obj.size,
				Metadata: obj.metadata,
			})
		}
	}
	return result, nil
}

// Head реализует получение метаданных объекта из in-memory хранилища.
func (s *fakeStorage) Head(ctx context.Context, bucket, key string) (*Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bucketObjs, ok := s.objects[bucket]
	if !ok {
		return nil, nil
	}

	obj, ok := bucketObjs[key]
	if !ok {
		return nil, nil
	}

	return &Object{
		Key:      key,
		Size:     obj.size,
		Metadata: obj.metadata,
	}, nil
}

// fakeWriterAt реализует io.WriterAt через файл для тестирования Download.
type fakeWriterAt struct {
	file *os.File
}

func (w *fakeWriterAt) WriteAt(p []byte, off int64) (int, error) {
	return w.file.WriteAt(p, off)
}
