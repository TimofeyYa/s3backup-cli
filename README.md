# s3back

CLI-утилита для резервного копирования директорий в S3-совместимое хранилище.

## Возможности

- **Интерактивная инициализация** — Bubble Tea wizard для настройки подключения к хранилищу
- **Резервное копирование** — упаковка директории в tar.gz и загрузка в S3 с метаданными и тегами
- **Восстановление** — скачивание и распаковка архива (по тегу или самого свежего)
- **Просмотр списка** — таблица всех архивов в бакете с фильтрацией
- **Поддержка symlink** — корректная обработка символических ссылок в архивах
- **Скрытые файлы** — включаются в архив
- **Настройка сжатия** — уровень gzip от 1 (быстро) до 9 (максимально)
- **Path-style URL** — совместимость с MinIO и другими S3-совместимыми хранилищами
- **Безопасное хранение конфига** — файл конфигурации создаётся с правами 0600

## Установка и сборка

**Требования:**
- Go 1.22+

```bash
# Клонировать репозиторий
git clone https://github.com/sypertimka/s3back.git
cd s3back

# Собрать
make build

# Бинарный файл будет в ./bin/s3back
./bin/s3back version
```

Для кросс-компиляции:

```bash
make release
# Бинарные файлы будут в ./dist/
```

## Быстрый старт

```bash
# 1. Инициализировать конфигурацию (интерактивный мастер)
s3back init

# 2. Создать резервную копию директории
s3back save ~/projects -tag daily -to my-backups

# 3. Просмотреть список архивов
s3back list -from my-backups

# 4. Восстановить последний архив
s3back download -from my-backups -to /tmp/restore
```

## Команды

### `s3back init`

Запускает интерактивный Bubble Tea wizard для настройки конфигурации. Последовательно запрашивает:

| Параметр | Описание | Обязательный |
|----------|----------|:---:|
| endpoint | URL S3-хранилища | да |
| region | Регион хранилища | да |
| access_key_id | Идентификатор ключа доступа | да |
| secret_access_key | Секретный ключ (скрытый ввод) | да |
| session_token | Токен сессии для временных ключей | нет |
| default_bucket | Бакет по умолчанию | нет |
| compression_level | Уровень сжатия gzip (1-9) | нет |
| path_style | Использовать path-style URL | нет |
| use_ssl | Использовать HTTPS | нет |

Конфигурация сохраняется в `~/.s3backup/config.yaml` с правами `0600`. При повторном запуске — обновляет существующий конфиг (текущие значения используются как defaults).

---

### `s3back save <path> -tag <tag> [-to <bucket>]`

Создаёт tar.gz архив директории и загружает в S3.

```bash
# Резервное копирование ~/data с тегом "weekly" в бакет backups
s3back save ~/data -tag weekly -to backups

# Использовать бакет из конфига (default_bucket)
s3back save ~/data -tag daily
```

**Флаги:**

| Флаг | Описание |
|------|----------|
| `-tag <tag>` | Тег для идентификации копии (обязателен) |
| `-to <bucket>` | Бакет назначения (по умолчанию — `default_bucket` из конфига) |

Имя ключа объекта в S3: `backups/<имя-директории>/<timestamp>__<tag>.tar.gz`

Пример: `backups/data/2024-06-15T10-30-00Z__weekly.tar.gz`

Метаданные объекта:
- `backup-tag` — тег
- `source-name` — нормализованное имя директории
- `created-at` — время создания (RFC3339)
- `app-version` — версия s3back

---

### `s3back download -from <bucket> [-tag <tag>] [-to <path>] [--force] [--dry-run]`

Скачивает архив из S3 и распаковывает его.

```bash
# Скачать самый свежий архив
s3back download -from my-backups -to /tmp/restore

# Скачать конкретный тег
s3back download -from my-backups -tag weekly -to /tmp/restore

# Перезаписать существующие файлы
s3back download -from my-backups -to /tmp/restore --force

# Проверить без выполнения
s3back download -from my-backups --dry-run
```

**Флаги:**

| Флаг | Описание |
|------|----------|
| `-from <bucket>` | Бакет источника (обязателен) |
| `-tag <tag>` | Тег архива (по умолчанию — самый свежий по timestamp) |
| `-to <path>` | Путь для распаковки (по умолчанию — текущая директория) |
| `--force` | Перезаписывать существующие файлы |
| `--dry-run` | Показать, что будет сделано, без выполнения |

---

### `s3back list -from <bucket> [--source <name>] [--dry-run]`

Выводит таблицу архивов в бакете.

```bash
# Все архивы в бакете
s3back list -from my-backups

# Только для источника "data"
s3back list -from my-backups --source data
```

**Выходная таблица:**

```
TIMESTAMP              TAG     SOURCE  SIZE      KEY
---------              ---     ------  ----      ---
2024-06-15T10-30-00Z   weekly  data    2.35 MB   backups/data/2024-06-15T10-30-00Z__weekly.tar.gz
2024-06-01T08-00-00Z   daily   data    1.10 MB   backups/data/2024-06-01T08-00-00Z__daily.tar.gz
```

---

### `s3back config show`

Выводит текущую конфигурацию. Секретные поля (`secret_access_key`, `session_token`) заменяются на `***`.

---

### `s3back config validate`

Проверяет валидность конфигурации (наличие обязательных полей).

---

### `s3back version`

Выводит версию приложения.

---

### `s3back help`

Выводит полную справку со всеми командами, флагами и примерами.

---

## Конфигурация

**Расположение:** `~/.s3backup/config.yaml`  
**Права:** `0600` (только владелец может читать и изменять)

```yaml
endpoint: "https://s3.amazonaws.com"
region: "us-east-1"
access_key_id: "YOUR_ACCESS_KEY"
secret_access_key: "YOUR_SECRET_KEY"
session_token: ""
default_bucket: "my-backups"
compression_level: 6
path_style: false
use_ssl: true
```

Пример конфигурации для разных провайдеров:

**AWS S3:**
```yaml
endpoint: "https://s3.amazonaws.com"
region: "us-east-1"
path_style: false
use_ssl: true
```

**MinIO:**
```yaml
endpoint: "http://localhost:9000"
region: "us-east-1"
path_style: true
use_ssl: false
```

**Yandex Object Storage:**
```yaml
endpoint: "https://storage.yandexcloud.net"
region: "ru-central1"
path_style: false
use_ssl: true
```

**Selectel:**
```yaml
endpoint: "https://s3.selcdn.ru"
region: "ru-1"
path_style: true
use_ssl: true
```

## Exit Codes

| Код | Описание |
|-----|----------|
| 0 | Успех |
| 1 | Общая ошибка |
| 2 | Ошибка валидации аргументов |
| 3 | Конфиг не найден (выполните `s3back init`) |
| 4 | Исходный путь не существует |
| 5 | Ошибка S3 (соединение или авторизация) |
| 6 | Объект не найден |
| 7 | Конфликт файлов при восстановлении |

## Архитектура

```
s3back/
├── cmd/s3back/main.go          # Точка входа
├── internal/
│   ├── cli/                    # CLI парсер и маршрутизатор команд
│   │   ├── cli.go              # Разбор аргументов, маршрутизация
│   │   ├── adapter.go          # Адаптер S3-клиента → usecases.Storage
│   │   └── help_test.go        # Golden-тесты CLI
│   ├── config/                 # Конфигурация
│   │   ├── config.go           # Загрузка/сохранение ~/.s3backup/config.yaml
│   │   └── config_test.go      # Тесты: Save/Load, права, ошибка при отсутствии
│   ├── archive/                # Работа с архивами
│   │   ├── archive.go          # tar.gz упаковка/распаковка
│   │   └── archive_test.go     # Тесты: roundtrip, неизменность источника, конфликт
│   ├── storage/s3/             # S3-клиент
│   │   └── client.go           # AWS SDK v2 обёртка (upload/download/list/head)
│   ├── app/usecases/           # Бизнес-логика
│   │   ├── storage.go          # Интерфейс Storage + ProgressFunc
│   │   ├── init.go             # Операция инициализации конфига
│   │   ├── save.go             # Операция резервного копирования
│   │   ├── download.go         # Операция скачивания и распаковки
│   │   ├── list.go             # Операция получения списка архивов
│   │   ├── save_test.go        # Тесты save (fake Storage)
│   │   └── download_test.go    # Тесты download (fake Storage)
│   ├── tui/                    # Bubble Tea интерфейс
│   │   ├── init.go             # Wizard инициализации (пошаговый ввод)
│   │   ├── progress.go         # Прогресс-бар для save/download
│   │   └── error.go            # Вывод ошибок (lipgloss)
│   └── restore/                # Зарезервирован для будущих расширений
└── go.mod

Поток данных:
  os.Args → cli.Run() → usecases.Save/Download/List()
                      → archive.Create/Extract()
                      → s3client.Upload/Download/List()
                      → AWS SDK v2 → S3-хранилище
```

## Тестирование

```bash
# Запустить все unit-тесты
make test

# или напрямую
go test -race -count=1 ./...

# Интеграционные тесты (требуют реального S3-хранилища)
make integration-test

# Только тесты конкретного пакета
go test -v ./internal/archive/
go test -v ./internal/config/
go test -v ./internal/app/usecases/
go test -v ./internal/cli/
```

**Покрытие тестами:**

| Пакет | Тесты |
|-------|-------|
| `archive` | roundtrip, неизменность источника, конфликт файлов |
| `config` | save/load, права файла, ошибка при отсутствии |
| `app/usecases` | save с правильным ключом/метаданными, download с тегом, выбор свежего |
| `cli` | вывод help, команда version, неизвестная команда |

## Поддерживаемые S3-совместимые провайдеры

- **AWS S3** — нативная поддержка
- **MinIO** — `path_style: true`
- **Yandex Object Storage** — `endpoint: https://storage.yandexcloud.net`
- **Selectel** — `endpoint: https://s3.selcdn.ru`
- **Cloudflare R2** — `endpoint: https://<account-id>.r2.cloudflarestorage.com`
- **Backblaze B2** — S3-совместимый endpoint
- **Любое S3-совместимое хранилище** — через настройку endpoint и path_style

## Лицензия

MIT — см. файл [LICENSE](LICENSE).
