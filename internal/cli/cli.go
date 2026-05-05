// Пакет cli реализует разбор аргументов командной строки и маршрутизацию команд.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/sypertimka/s3back/internal/app/usecases"
	"github.com/sypertimka/s3back/internal/config"
	s3client "github.com/sypertimka/s3back/internal/storage/s3"
	"github.com/sypertimka/s3back/internal/tui"
)

// Константы для exit кодов.
const (
	ExitSuccess         = 0 // Успех
	ExitError           = 1 // Общая ошибка
	ExitArgError        = 2 // Ошибка валидации аргументов
	ExitConfigNotFound  = 3 // Конфиг не найден
	ExitSourceNotExist  = 4 // Исходный путь не существует
	ExitS3Error         = 5 // Ошибка S3
	ExitNotFound        = 6 // Объект не найден
	ExitFileConflict    = 7 // Конфликт файлов при восстановлении
)

// Version — версия приложения. Устанавливается через ldflags при сборке.
var Version = "0.1.0"

// Run является точкой входа CLI. Разбирает os.Args и маршрутизирует команды.
// Возвращает exit code.
func Run(args []string) int {
	if len(args) < 1 {
		printUsage()
		return ExitArgError
	}

	command := args[0]
	remaining := args[1:]

	switch command {
	case "init":
		return cmdInit(remaining)
	case "save":
		return cmdSave(remaining)
	case "download":
		return cmdDownload(remaining)
	case "list":
		return cmdList(remaining)
	case "config":
		return cmdConfig(remaining)
	case "version":
		fmt.Printf("s3back version %s\n", Version)
		return ExitSuccess
	case "help", "--help", "-h":
		printHelp()
		return ExitSuccess
	default:
		fmt.Fprintf(os.Stderr, "неизвестная команда: %s\n\n", command)
		printUsage()
		return ExitArgError
	}
}

// cmdInit обрабатывает команду инициализации.
// Запускает Bubble Tea wizard для ввода параметров конфигурации.
func cmdInit(_ []string) int {
	// Загружаем существующий конфиг (если есть) для подстановки defaults
	existing := usecases.LoadExistingConfig()

	// Проверяем, является ли терминал TTY
	if !isTerminal() {
		// Не-TTY режим: используем простой текстовый ввод
		fmt.Println("Терминал не является TTY. Используйте переменные окружения или отредактируйте конфиг напрямую.")
		return ExitError
	}

	// Запускаем Bubble Tea wizard
	model := tui.NewInitWizardModel(existing)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		tui.PrintErrorf("ошибка запуска мастера: %v", err)
		return ExitError
	}

	wizardModel := finalModel.(tui.InitWizardModel)
	if !wizardModel.IsDone() {
		fmt.Println("Инициализация отменена.")
		return ExitSuccess
	}

	// Сохраняем конфигурацию
	result := wizardModel.Result()
	if result == nil {
		return ExitSuccess
	}

	if err := usecases.Init(usecases.InitParams{Config: result}); err != nil {
		tui.PrintErrorf("не удалось сохранить конфиг: %v", err)
		return ExitError
	}

	p2, err := config.Path()
	if err == nil {
		fmt.Printf("Конфигурация сохранена в %s\n", p2)
	}

	return ExitSuccess
}

// cmdSave обрабатывает команду резервного копирования.
func cmdSave(args []string) int {
	fs := flag.NewFlagSet("save", flag.ContinueOnError)
	toBucket := fs.String("to", "", "имя S3-бакета назначения")
	tag := fs.String("tag", "", "тег для идентификации резервной копии (обязателен)")

	if err := fs.Parse(args); err != nil {
		return ExitArgError
	}

	// Проверяем наличие пути
	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "укажите путь для резервного копирования: s3back save <path> -tag <tag>")
		return ExitArgError
	}
	srcPath := fs.Arg(0)

	// Проверяем тег
	if *tag == "" {
		fmt.Fprintln(os.Stderr, "укажите тег: -tag <tag>")
		return ExitArgError
	}

	// Загружаем конфиг
	cfg, exitCode := loadConfig()
	if exitCode != ExitSuccess {
		return exitCode
	}

	// Определяем бакет
	bucket := *toBucket
	if bucket == "" {
		bucket = cfg.DefaultBucket
	}
	if bucket == "" {
		fmt.Fprintln(os.Stderr, "укажите имя bucket через -to или сохраните default_bucket в конфиге")
		return ExitArgError
	}

	// Проверяем существование пути
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "исходный путь не существует: %s\n", srcPath)
		return ExitSourceNotExist
	}

	// Создаём S3-клиент
	client, err := s3client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "не удалось подключиться к S3: %v\n", err)
		return ExitS3Error
	}

	// Адаптер для интерфейса Storage
	storage := &s3StorageAdapter{client: client}

	ctx := context.Background()

	fmt.Printf("Создание резервной копии: %s → %s (тег: %s)\n", srcPath, bucket, *tag)

	result, err := usecases.Save(ctx, storage, usecases.SaveParams{
		SourcePath:       srcPath,
		Bucket:           bucket,
		Tag:              *tag,
		CompressionLevel: cfg.CompressionLevel,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		if isS3Error(err) {
			return ExitS3Error
		}
		if isSourceError(err) {
			return ExitSourceNotExist
		}
		return ExitError
	}

	fmt.Printf("✓ Резервная копия создана успешно!\n")
	fmt.Printf("  Ключ: %s\n", result.Key)
	fmt.Printf("  Размер: %s\n", formatSize(result.ArchiveSize))
	fmt.Printf("  Время: %v\n", result.Duration.Round(1e6))

	return ExitSuccess
}

// cmdDownload обрабатывает команду скачивания архива.
func cmdDownload(args []string) int {
	fs := flag.NewFlagSet("download", flag.ContinueOnError)
	fromBucket := fs.String("from", "", "имя S3-бакета источника (обязателен)")
	tag := fs.String("tag", "", "тег архива (опционально, по умолчанию — самый свежий)")
	toPath := fs.String("to", "", "путь для распаковки (по умолчанию — текущая директория)")
	force := fs.Bool("force", false, "перезаписывать существующие файлы")
	dryRun := fs.Bool("dry-run", false, "показать, что будет сделано, без выполнения")

	if err := fs.Parse(args); err != nil {
		return ExitArgError
	}

	if *fromBucket == "" {
		fmt.Fprintln(os.Stderr, "укажите бакет: -from <bucket>")
		return ExitArgError
	}

	// Загружаем конфиг
	cfg, exitCode := loadConfig()
	if exitCode != ExitSuccess {
		return exitCode
	}

	// Создаём S3-клиент
	client, err := s3client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "не удалось подключиться к S3: %v\n", err)
		return ExitS3Error
	}

	storage := &s3StorageAdapter{client: client}
	ctx := context.Background()

	result, err := usecases.Download(ctx, storage, usecases.DownloadParams{
		Bucket:   *fromBucket,
		Tag:      *tag,
		DestPath: *toPath,
		Force:    *force,
		DryRun:   *dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		if isFileConflict(err) {
			return ExitFileConflict
		}
		if isNotFoundError(err) {
			return ExitNotFound
		}
		if isS3Error(err) {
			return ExitS3Error
		}
		return ExitError
	}

	if !*dryRun {
		fmt.Printf("✓ Архив распакован успешно!\n")
		fmt.Printf("  Ключ: %s\n", result.Key)
		fmt.Printf("  Путь: %s\n", result.DestPath)
		fmt.Printf("  Размер: %s\n", formatSize(result.Size))
	}

	return ExitSuccess
}

// cmdList обрабатывает команду вывода списка архивов.
func cmdList(args []string) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fromBucket := fs.String("from", "", "имя S3-бакета (обязателен)")
	source := fs.String("source", "", "фильтр по имени источника (опционально)")
	dryRun := fs.Bool("dry-run", false, "только вывести запрос без выполнения")

	if err := fs.Parse(args); err != nil {
		return ExitArgError
	}

	if *fromBucket == "" {
		fmt.Fprintln(os.Stderr, "укажите бакет: -from <bucket>")
		return ExitArgError
	}

	// Загружаем конфиг
	cfg, exitCode := loadConfig()
	if exitCode != ExitSuccess {
		return exitCode
	}

	// Создаём S3-клиент
	client, err := s3client.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "не удалось подключиться к S3: %v\n", err)
		return ExitS3Error
	}

	storage := &s3StorageAdapter{client: client}
	ctx := context.Background()

	items, err := usecases.List(ctx, storage, usecases.ListParams{
		Bucket: *fromBucket,
		Source: *source,
		DryRun: *dryRun,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		if isS3Error(err) {
			return ExitS3Error
		}
		return ExitError
	}

	if len(items) == 0 {
		fmt.Println("Архивы не найдены.")
		return ExitSuccess
	}

	// Выводим таблицу
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tTAG\tSOURCE\tSIZE\tKEY")
	fmt.Fprintln(w, "---------\t---\t------\t----\t---")
	for _, item := range items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			item.Timestamp,
			item.Tag,
			item.Source,
			formatSize(item.Size),
			item.Key,
		)
	}
	w.Flush()

	return ExitSuccess
}

// cmdConfig обрабатывает подкоманды config show и config validate.
func cmdConfig(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "укажите подкоманду: show | validate")
		return ExitArgError
	}

	switch args[0] {
	case "show":
		return cmdConfigShow()
	case "validate":
		return cmdConfigValidate()
	default:
		fmt.Fprintf(os.Stderr, "неизвестная подкоманда config: %s\n", args[0])
		return ExitArgError
	}
}

// cmdConfigShow выводит текущий конфиг без секретных данных.
func cmdConfigShow() int {
	cfg, exitCode := loadConfig()
	if exitCode != ExitSuccess {
		return exitCode
	}

	p, _ := config.Path()
	fmt.Printf("Конфигурация (%s):\n\n", p)
	fmt.Printf("  endpoint:          %s\n", cfg.Endpoint)
	fmt.Printf("  region:            %s\n", cfg.Region)
	fmt.Printf("  access_key_id:     %s\n", cfg.AccessKeyID)
	fmt.Printf("  secret_access_key: %s\n", "***")
	if cfg.SessionToken != "" {
		fmt.Printf("  session_token:     %s\n", "***")
	} else {
		fmt.Printf("  session_token:     (не задан)\n")
	}
	fmt.Printf("  default_bucket:    %s\n", cfg.DefaultBucket)
	fmt.Printf("  compression_level: %d\n", cfg.CompressionLevel)
	fmt.Printf("  path_style:        %v\n", cfg.PathStyle)
	fmt.Printf("  use_ssl:           %v\n", cfg.UseSSL)

	return ExitSuccess
}

// cmdConfigValidate проверяет валидность текущего конфига.
func cmdConfigValidate() int {
	cfg, exitCode := loadConfig()
	if exitCode != ExitSuccess {
		return exitCode
	}

	errs := config.Validate(cfg)
	if len(errs) == 0 {
		fmt.Println("✓ Конфигурация валидна.")
		return ExitSuccess
	}

	fmt.Fprintln(os.Stderr, "Ошибки конфигурации:")
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "  - %s\n", e)
	}
	return ExitError
}

// loadConfig загружает конфигурацию и возвращает exit code при ошибке.
func loadConfig() (*config.Config, int) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return nil, ExitConfigNotFound
	}
	return cfg, ExitSuccess
}

// isTerminal проверяет, является ли stdout терминалом (TTY).
func isTerminal() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// formatSize форматирует размер в человекочитаемый вид.
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// isS3Error проверяет, является ли ошибка S3-ошибкой.
func isS3Error(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "S3") || contains(msg, "подключиться") || contains(msg, "aws") || contains(msg, "credential")
}

// isSourceError проверяет, является ли ошибка ошибкой источника.
func isSourceError(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "не существует")
}

// isFileConflict проверяет, является ли ошибка конфликтом файлов.
func isFileConflict(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "уже существует")
}

// isNotFoundError проверяет, является ли ошибка "объект не найден".
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "не найден") || contains(err.Error(), "нет архивов")
}

// contains проверяет вхождение подстроки (без учёта регистра не нужно здесь).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// printUsage выводит краткую справку по использованию.
func printUsage() {
	fmt.Println("Использование: s3back <команда> [флаги]")
	fmt.Println()
	fmt.Println("Команды:")
	fmt.Println("  init              Интерактивная настройка конфигурации")
	fmt.Println("  save              Создать резервную копию директории")
	fmt.Println("  download          Скачать и распаковать архив")
	fmt.Println("  list              Список архивов в бакете")
	fmt.Println("  config show       Показать текущий конфиг (без секретов)")
	fmt.Println("  config validate   Проверить валидность конфига")
	fmt.Println("  version           Версия приложения")
	fmt.Println("  help              Подробная справка")
	fmt.Println()
	fmt.Println("Запустите 's3back help' для подробной информации.")
}

// printHelp выводит полную справку.
func printHelp() {
	fmt.Printf(`s3back — утилита резервного копирования директорий в S3-совместимое хранилище
Версия: %s

ИСПОЛЬЗОВАНИЕ:
  s3back <команда> [флаги] [аргументы]

КОМАНДЫ:
  init
    Запустить интерактивный мастер настройки конфигурации.
    Сохраняет конфиг в ~/.s3backup/config.yaml (права 0600).
    При повторном запуске — обновляет существующий конфиг.

  save <path> -tag <tag> [-to <bucket>]
    Создать резервную копию директории <path>.
    Флаги:
      -tag <tag>      Тег для идентификации копии (обязателен)
      -to <bucket>    Бакет назначения (по умолчанию из конфига)

  download -from <bucket> [-tag <tag>] [-to <path>] [--force] [--dry-run]
    Скачать архив из бакета и распаковать.
    Флаги:
      -from <bucket>  Бакет источника (обязателен)
      -tag <tag>      Тег архива (по умолчанию — самый свежий)
      -to <path>      Путь для распаковки (по умолчанию — текущая директория)
      --force         Перезаписывать существующие файлы
      --dry-run       Показать, что будет сделано, без выполнения

  list -from <bucket> [--source <name>] [--dry-run]
    Вывести список архивов в бакете в виде таблицы.
    Флаги:
      -from <bucket>     Бакет (обязателен)
      --source <name>    Фильтр по имени источника
      --dry-run          Показать запрос без выполнения

  config show
    Показать текущий конфиг (секретные поля заменены на ***)

  config validate
    Проверить валидность текущего конфига

  version
    Вывести версию приложения

  help
    Показать эту справку

ПРИМЕРЫ:
  # Инициализация конфигурации
  s3back init

  # Резервное копирование директории ~/projects с тегом "daily"
  s3back save ~/projects -tag daily -to my-backups

  # Просмотр всех архивов
  s3back list -from my-backups

  # Скачать самый свежий архив
  s3back download -from my-backups -to /tmp/restore

  # Скачать конкретный тег
  s3back download -from my-backups -tag daily -to /tmp/restore --force

  # Проверить конфиг
  s3back config validate

ПЕРЕМЕННЫЕ ОКРУЖЕНИЯ:
  HOME    Домашняя директория (для поиска конфига)

КОНФИГУРАЦИЯ:
  Файл: ~/.s3backup/config.yaml
  Права: 0600 (только для владельца)

EXIT CODES:
  0   Успех
  1   Общая ошибка
  2   Ошибка валидации аргументов
  3   Конфиг не найден (выполните s3back init)
  4   Исходный путь не существует
  5   Ошибка S3 (соединение/авторизация)
  6   Объект не найден
  7   Конфликт файлов при восстановлении

ПОДДЕРЖИВАЕМЫЕ ХРАНИЛИЩА:
  AWS S3, MinIO, Yandex Object Storage, Selectel, любое S3-совместимое хранилище

`, Version)
}
