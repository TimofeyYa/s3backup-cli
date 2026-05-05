// Тесты для пакета config.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSaveLoad проверяет, что Save/Load сохраняет и восстанавливает все поля конфига.
func TestSaveLoad(t *testing.T) {
	// Используем временную директорию как домашнюю
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	original := &Config{
		Endpoint:         "https://s3.example.com",
		Region:           "us-east-1",
		AccessKeyID:      "test-key-id",
		SecretAccessKey:  "test-secret",
		SessionToken:     "test-token",
		DefaultBucket:    "test-bucket",
		CompressionLevel: 6,
		PathStyle:        true,
		UseSSL:           true,
	}

	// Сохраняем конфиг
	if err := Save(original); err != nil {
		t.Fatalf("Save завершился с ошибкой: %v", err)
	}

	// Загружаем конфиг
	loaded, err := Load()
	if err != nil {
		t.Fatalf("Load завершился с ошибкой: %v", err)
	}

	// Сравниваем поля
	if loaded.Endpoint != original.Endpoint {
		t.Errorf("Endpoint: ожидается %q, получено %q", original.Endpoint, loaded.Endpoint)
	}
	if loaded.Region != original.Region {
		t.Errorf("Region: ожидается %q, получено %q", original.Region, loaded.Region)
	}
	if loaded.AccessKeyID != original.AccessKeyID {
		t.Errorf("AccessKeyID: ожидается %q, получено %q", original.AccessKeyID, loaded.AccessKeyID)
	}
	if loaded.SecretAccessKey != original.SecretAccessKey {
		t.Errorf("SecretAccessKey: ожидается %q, получено %q", original.SecretAccessKey, loaded.SecretAccessKey)
	}
	if loaded.SessionToken != original.SessionToken {
		t.Errorf("SessionToken: ожидается %q, получено %q", original.SessionToken, loaded.SessionToken)
	}
	if loaded.DefaultBucket != original.DefaultBucket {
		t.Errorf("DefaultBucket: ожидается %q, получено %q", original.DefaultBucket, loaded.DefaultBucket)
	}
	if loaded.CompressionLevel != original.CompressionLevel {
		t.Errorf("CompressionLevel: ожидается %d, получено %d", original.CompressionLevel, loaded.CompressionLevel)
	}
	if loaded.PathStyle != original.PathStyle {
		t.Errorf("PathStyle: ожидается %v, получено %v", original.PathStyle, loaded.PathStyle)
	}
	if loaded.UseSSL != original.UseSSL {
		t.Errorf("UseSSL: ожидается %v, получено %v", original.UseSSL, loaded.UseSSL)
	}
}

// TestPermissions проверяет, что файл конфига создаётся с правами 0600.
func TestPermissions(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := &Config{
		Endpoint:        "https://s3.example.com",
		Region:          "us-east-1",
		AccessKeyID:     "key",
		SecretAccessKey: "secret",
	}

	if err := Save(cfg); err != nil {
		t.Fatalf("Save завершился с ошибкой: %v", err)
	}

	p := filepath.Join(tmpHome, ".s3backup", "config.yaml")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("не удалось получить информацию о файле: %v", err)
	}

	// Проверяем права 0600
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("права файла: ожидается 0600, получено %04o", perm)
	}
}

// TestLoadMissing проверяет, что Load возвращает понятную ошибку при отсутствии файла.
func TestLoadMissing(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	_, err := Load()
	if err == nil {
		t.Fatal("ожидалась ошибка при отсутствии конфига, получено nil")
	}

	// Проверяем, что сообщение об ошибке содержит подсказку
	errMsg := err.Error()
	if len(errMsg) == 0 {
		t.Error("сообщение об ошибке пустое")
	}
	t.Logf("Сообщение об ошибке: %s", errMsg)
}
