// Пакет config отвечает за загрузку и сохранение конфигурации приложения.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config хранит все настройки подключения к S3-совместимому хранилищу.
type Config struct {
	// Endpoint — URL S3-совместимого хранилища (например, https://s3.amazonaws.com)
	Endpoint string `yaml:"endpoint"`
	// Region — регион хранилища
	Region string `yaml:"region"`
	// AccessKeyID — идентификатор ключа доступа
	AccessKeyID string `yaml:"access_key_id"`
	// SecretAccessKey — секретный ключ доступа
	SecretAccessKey string `yaml:"secret_access_key"`
	// SessionToken — токен сессии (опционально, для временных учётных данных)
	SessionToken string `yaml:"session_token,omitempty"`
	// DefaultBucket — бакет по умолчанию для операций save/download
	DefaultBucket string `yaml:"default_bucket,omitempty"`
	// CompressionLevel — уровень сжатия gzip (1-9, по умолчанию 6)
	CompressionLevel int `yaml:"compression_level,omitempty"`
	// PathStyle — использовать path-style URL (для MinIO и других совместимых хранилищ)
	PathStyle bool `yaml:"path_style"`
	// UseSSL — использовать HTTPS
	UseSSL bool `yaml:"use_ssl"`
}

// configDir возвращает путь к директории конфигурации.
func configDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("не удалось определить домашнюю директорию: %w", err)
	}
	return filepath.Join(home, ".s3backup"), nil
}

// Path возвращает полный путь к файлу конфигурации.
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// Exists проверяет наличие файла конфигурации.
func Exists() bool {
	p, err := Path()
	if err != nil {
		return false
	}
	_, err = os.Stat(p)
	return err == nil
}

// Load загружает конфигурацию из файла ~/.s3backup/config.yaml.
// Возвращает ошибку, если файл не существует или не может быть прочитан.
func Load() (*Config, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("конфиг не найден, выполните: s3back init")
		}
		return nil, fmt.Errorf("не удалось прочитать конфиг: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("не удалось разобрать конфиг: %w", err)
	}

	// Устанавливаем значение по умолчанию для уровня сжатия
	if cfg.CompressionLevel == 0 {
		cfg.CompressionLevel = 6
	}

	return &cfg, nil
}

// Save сохраняет конфигурацию в файл ~/.s3backup/config.yaml с правами 0600.
// Директория создаётся автоматически с правами 0700.
func Save(c *Config) error {
	dir, err := configDir()
	if err != nil {
		return err
	}

	// Создаём директорию конфига с правами 0700
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("не удалось создать директорию конфига: %w", err)
	}

	// Устанавливаем значение по умолчанию для уровня сжатия
	if c.CompressionLevel == 0 {
		c.CompressionLevel = 6
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("не удалось сериализовать конфиг: %w", err)
	}

	p, err := Path()
	if err != nil {
		return err
	}

	// Записываем с правами 0600 (только владелец может читать/писать)
	if err := os.WriteFile(p, data, 0600); err != nil {
		return fmt.Errorf("не удалось записать конфиг: %w", err)
	}

	return nil
}

// Validate проверяет валидность конфигурации.
// Возвращает список ошибок (если они есть).
func Validate(c *Config) []string {
	var errs []string
	if c.Endpoint == "" {
		errs = append(errs, "endpoint не задан")
	}
	if c.Region == "" {
		errs = append(errs, "region не задан")
	}
	if c.AccessKeyID == "" {
		errs = append(errs, "access_key_id не задан")
	}
	if c.SecretAccessKey == "" {
		errs = append(errs, "secret_access_key не задан")
	}
	if c.CompressionLevel < 1 || c.CompressionLevel > 9 {
		if c.CompressionLevel != 0 {
			errs = append(errs, "compression_level должен быть от 1 до 9")
		}
	}
	return errs
}
