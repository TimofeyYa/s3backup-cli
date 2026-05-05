// Логика операции инициализации конфигурации.
package usecases

import (
	"github.com/sypertimka/s3back/internal/config"
)

// InitParams содержит параметры для инициализации конфигурации.
type InitParams struct {
	// Config — конфигурация для сохранения
	Config *config.Config
}

// Init сохраняет конфигурацию в файл.
// Если конфиг уже существует, обновляет его (загружает текущий и обновляет поля).
func Init(params InitParams) error {
	return config.Save(params.Config)
}

// LoadExistingConfig загружает текущую конфигурацию, если она существует.
// Возвращает пустой конфиг с дефолтными значениями, если файл не существует.
func LoadExistingConfig() *config.Config {
	if !config.Exists() {
		return &config.Config{
			CompressionLevel: 6,
			UseSSL:           true,
		}
	}
	cfg, err := config.Load()
	if err != nil {
		return &config.Config{
			CompressionLevel: 6,
			UseSSL:           true,
		}
	}
	return cfg
}
