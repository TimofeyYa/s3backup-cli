// Модель прогресс-бара для операций save и download.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
)

// ProgressMsg — сообщение для обновления прогресса.
type ProgressMsg float64

// DoneMsg — сообщение о завершении операции.
type DoneMsg struct{}

// ProgressModel — Bubble Tea модель для отображения прогресс-бара.
type ProgressModel struct {
	// progress — компонент прогресс-бара
	progress progress.Model
	// percent — текущий процент завершения (0.0 - 1.0)
	percent float64
	// operation — название текущей операции
	operation string
	// done — признак завершения операции
	done bool
}

// NewProgressModel создаёт новую модель прогресс-бара.
func NewProgressModel(operation string) ProgressModel {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)
	return ProgressModel{
		progress:  p,
		operation: operation,
	}
}

// Init инициализирует модель (Bubble Tea интерфейс).
func (m ProgressModel) Init() tea.Cmd {
	return nil
}

// Update обрабатывает входящие сообщения (Bubble Tea интерфейс).
func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ProgressMsg:
		// Обновляем прогресс
		m.percent = float64(msg)
		if m.percent >= 1.0 {
			m.done = true
			return m, tea.Quit
		}
		return m, nil

	case DoneMsg:
		m.percent = 1.0
		m.done = true
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

	case progress.FrameMsg:
		progressModel, cmd := m.progress.Update(msg)
		m.progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

// View отрисовывает текущее состояние модели (Bubble Tea интерфейс).
func (m ProgressModel) View() string {
	pad := strings.Repeat(" ", 2)
	if m.done {
		return pad + m.progress.ViewAs(1.0) + "\n" + pad + fmt.Sprintf("%s завершено!\n", m.operation)
	}
	return pad + m.progress.ViewAs(m.percent) + "\n" + pad + fmt.Sprintf("%s... %.0f%%\n", m.operation, m.percent*100)
}
