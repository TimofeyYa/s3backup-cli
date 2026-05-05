// Пакет tui содержит Bubble Tea модели для интерактивного терминального интерфейса.
package tui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

// Стили для отображения ошибок.
var (
	// errorStyle — стиль для текста ошибки (красный цвет)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true)

	// labelStyle — стиль для метки ошибки
	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true).
			SetString("Ошибка: ")
)

// PrintError выводит сообщение об ошибке красным цветом в stderr.
func PrintError(msg string) {
	fmt.Fprintln(os.Stderr, labelStyle.String()+errorStyle.Render(msg))
}

// PrintErrorf выводит отформатированное сообщение об ошибке в stderr.
func PrintErrorf(format string, args ...interface{}) {
	PrintError(fmt.Sprintf(format, args...))
}

// successStyle — стиль для успешных сообщений (зелёный цвет)
var successStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("10")).
	Bold(true)

// PrintSuccess выводит сообщение об успехе зелёным цветом.
func PrintSuccess(msg string) {
	fmt.Println(successStyle.Render("✓ " + msg))
}

// infoStyle — стиль для информационных сообщений
var infoStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("12"))

// PrintInfo выводит информационное сообщение.
func PrintInfo(msg string) {
	fmt.Println(infoStyle.Render(msg))
}
