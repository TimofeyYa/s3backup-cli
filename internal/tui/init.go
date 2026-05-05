// Модель мастера инициализации (wizard) для команды s3back init.
package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/sypertimka/s3back/internal/config"
)

// Шаги мастера инициализации.
const (
	stepEndpoint         = iota // URL endpoint
	stepRegion                  // Регион
	stepAccessKeyID             // Идентификатор ключа доступа
	stepSecretAccessKey         // Секретный ключ доступа
	stepSessionToken            // Токен сессии (опционально)
	stepDefaultBucket           // Бакет по умолчанию (опционально)
	stepCompressionLevel        // Уровень сжатия (1-9)
	stepPathStyle               // Использовать path style URL
	stepUseSSL                  // Использовать SSL
	stepDone                    // Завершение
)

// Стили для wizard'а.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("12")).
			MarginBottom(1)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Bold(true)

	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Italic(true)

	doneStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("10"))
)

// InitWizardModel — Bubble Tea модель мастера инициализации конфигурации.
type InitWizardModel struct {
	// inputs — поля ввода для каждого шага
	inputs []textinput.Model
	// currentStep — текущий шаг мастера
	currentStep int
	// result — результирующая конфигурация
	result *config.Config
	// existing — существующая конфигурация (для defaults)
	existing *config.Config
	// done — признак завершения wizard'а
	done bool
	// quitting — признак отмены
	quitting bool
	// err — ошибка валидации
	err string
}

// stepConfig описывает параметры каждого шага мастера.
type stepConfig struct {
	label       string
	placeholder string
	isPassword  bool
	hint        string
}

// steps содержит конфигурацию всех шагов мастера.
var steps = []stepConfig{
	{
		label:       "Endpoint S3 хранилища",
		placeholder: "https://s3.amazonaws.com",
		hint:        "URL вашего S3-совместимого хранилища",
	},
	{
		label:       "Регион",
		placeholder: "us-east-1",
		hint:        "Регион хранилища (например: us-east-1, ru-central1)",
	},
	{
		label:       "Access Key ID",
		placeholder: "AKIAIOSFODNN7EXAMPLE",
		hint:        "Идентификатор ключа доступа",
	},
	{
		label:      "Secret Access Key",
		isPassword: true,
		hint:       "Секретный ключ доступа (не отображается)",
	},
	{
		label:       "Session Token",
		placeholder: "(пропустить — нажмите Enter)",
		hint:        "Токен сессии для временных учётных данных (опционально)",
	},
	{
		label:       "Бакет по умолчанию",
		placeholder: "(пропустить — нажмите Enter)",
		hint:        "Бакет для команд save/download без флага -to/-from",
	},
	{
		label:       "Уровень сжатия (1-9)",
		placeholder: "6",
		hint:        "Уровень gzip сжатия: 1 — быстро, 9 — максимально. По умолчанию: 6",
	},
	{
		label:       "Path style URL (true/false)",
		placeholder: "false",
		hint:        "Использовать path-style URL (нужен для MinIO и некоторых других хранилищ)",
	},
	{
		label:       "Использовать SSL (true/false)",
		placeholder: "true",
		hint:        "Использовать HTTPS для подключения к хранилищу",
	},
}

// NewInitWizardModel создаёт новую модель мастера инициализации.
// Если передан существующий конфиг — использует его значения как defaults.
func NewInitWizardModel(existing *config.Config) InitWizardModel {
	inputs := make([]textinput.Model, len(steps))

	for i, step := range steps {
		ti := textinput.New()
		ti.Placeholder = step.placeholder
		ti.CharLimit = 256

		if step.isPassword {
			ti.EchoMode = textinput.EchoPassword
			ti.EchoCharacter = '•'
		}

		// Подставляем текущие значения как defaults
		if existing != nil {
			switch i {
			case stepEndpoint:
				ti.SetValue(existing.Endpoint)
			case stepRegion:
				ti.SetValue(existing.Region)
			case stepAccessKeyID:
				ti.SetValue(existing.AccessKeyID)
			case stepSecretAccessKey:
				ti.SetValue(existing.SecretAccessKey)
			case stepSessionToken:
				ti.SetValue(existing.SessionToken)
			case stepDefaultBucket:
				ti.SetValue(existing.DefaultBucket)
			case stepCompressionLevel:
				if existing.CompressionLevel > 0 {
					ti.SetValue(strconv.Itoa(existing.CompressionLevel))
				}
			case stepPathStyle:
				ti.SetValue(fmt.Sprintf("%v", existing.PathStyle))
			case stepUseSSL:
				ti.SetValue(fmt.Sprintf("%v", existing.UseSSL))
			}
		}

		inputs[i] = ti
	}

	// Активируем первое поле ввода
	inputs[0].Focus()

	return InitWizardModel{
		inputs:   inputs,
		existing: existing,
		result:   &config.Config{},
	}
}

// Init инициализирует модель (Bubble Tea интерфейс).
func (m InitWizardModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update обрабатывает входящие сообщения (Bubble Tea интерфейс).
func (m InitWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.done || m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.quitting = true
			return m, tea.Quit

		case tea.KeyEnter:
			// Очищаем ошибку
			m.err = ""

			// Получаем введённое значение
			value := strings.TrimSpace(m.inputs[m.currentStep].Value())

			// Валидируем и сохраняем значение
			if err := m.setStepValue(m.currentStep, value); err != nil {
				m.err = err.Error()
				return m, nil
			}

			// Переходим к следующему шагу
			m.inputs[m.currentStep].Blur()
			m.currentStep++

			if m.currentStep >= len(steps) {
				// Всё введено — завершаем
				m.done = true
				return m, tea.Quit
			}

			// Активируем следующее поле
			m.inputs[m.currentStep].Focus()
			return m, textinput.Blink
		}
	}

	// Передаём событие текущему полю ввода
	var cmd tea.Cmd
	m.inputs[m.currentStep], cmd = m.inputs[m.currentStep].Update(msg)
	return m, cmd
}

// setStepValue сохраняет значение для текущего шага.
func (m *InitWizardModel) setStepValue(step int, value string) error {
	switch step {
	case stepEndpoint:
		if value == "" {
			return fmt.Errorf("endpoint не может быть пустым")
		}
		m.result.Endpoint = value
	case stepRegion:
		if value == "" {
			return fmt.Errorf("region не может быть пустым")
		}
		m.result.Region = value
	case stepAccessKeyID:
		if value == "" {
			return fmt.Errorf("access key ID не может быть пустым")
		}
		m.result.AccessKeyID = value
	case stepSecretAccessKey:
		if value == "" {
			return fmt.Errorf("secret access key не может быть пустым")
		}
		m.result.SecretAccessKey = value
	case stepSessionToken:
		m.result.SessionToken = value // опционально
	case stepDefaultBucket:
		m.result.DefaultBucket = value // опционально
	case stepCompressionLevel:
		if value == "" {
			m.result.CompressionLevel = 6
		} else {
			level, err := strconv.Atoi(value)
			if err != nil || level < 1 || level > 9 {
				return fmt.Errorf("уровень сжатия должен быть числом от 1 до 9")
			}
			m.result.CompressionLevel = level
		}
	case stepPathStyle:
		if value == "" {
			value = "false"
		}
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("введите true или false")
		}
		m.result.PathStyle = b
	case stepUseSSL:
		if value == "" {
			value = "true"
		}
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("введите true или false")
		}
		m.result.UseSSL = b
	}
	return nil
}

// View отрисовывает текущее состояние мастера (Bubble Tea интерфейс).
func (m InitWizardModel) View() string {
	if m.quitting {
		return "Инициализация отменена.\n"
	}
	if m.done {
		return doneStyle.Render("✓ Конфигурация сохранена успешно!") + "\n"
	}

	var sb strings.Builder

	// Заголовок
	sb.WriteString(titleStyle.Render("s3back — Мастер инициализации конфигурации") + "\n\n")

	// Прогресс шагов
	sb.WriteString(hintStyle.Render(fmt.Sprintf("Шаг %d из %d", m.currentStep+1, len(steps))) + "\n\n")

	// Текущий шаг
	step := steps[m.currentStep]
	sb.WriteString(promptStyle.Render(step.label+":") + "\n")
	sb.WriteString(m.inputs[m.currentStep].View() + "\n")
	sb.WriteString(hintStyle.Render(step.hint) + "\n")

	// Ошибка валидации
	if m.err != "" {
		sb.WriteString("\n" + errorStyle.Render("⚠ "+m.err) + "\n")
	}

	sb.WriteString("\n" + hintStyle.Render("Enter — следующий шаг, Esc — отмена") + "\n")

	return sb.String()
}

// Result возвращает результирующую конфигурацию.
// Возвращает nil, если wizard был отменён.
func (m InitWizardModel) Result() *config.Config {
	if m.quitting {
		return nil
	}
	return m.result
}

// IsDone возвращает true, если мастер завершён успешно.
func (m InitWizardModel) IsDone() bool {
	return m.done
}
