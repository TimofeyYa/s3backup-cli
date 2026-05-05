// Golden-тест для команды help: проверяет наличие ключевых слов в выводе.
package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// TestHelpContainsKeywords проверяет, что вывод команды help содержит
// все ключевые слова: команды, флаги, примеры.
func TestHelpContainsKeywords(t *testing.T) {
	// Перехватываем stdout
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("не удалось создать pipe: %v", err)
	}
	os.Stdout = w

	// Запускаем команду help
	Run([]string{"help"})

	// Восстанавливаем stdout
	w.Close()
	os.Stdout = oldStdout

	// Читаем перехваченный вывод
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("не удалось прочитать вывод: %v", err)
	}
	output := buf.String()

	// Проверяем наличие ключевых слов
	keywords := []string{
		"init",
		"save",
		"download",
		"list",
		"version",
		"config",
		"-tag",
		"-from",
		"-to",
		"--force",
		"--dry-run",
		"ПРИМЕРЫ",
		"EXIT CODES",
		"0",
		"1",
		"2",
		"3",
		"4",
		"5",
		"6",
		"7",
	}

	for _, kw := range keywords {
		if !strings.Contains(output, kw) {
			t.Errorf("вывод help не содержит ключевое слово: %q", kw)
		}
	}
}

// TestVersionCommand проверяет корректный вывод команды version.
func TestVersionCommand(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("не удалось создать pipe: %v", err)
	}
	os.Stdout = w

	exitCode := Run([]string{"version"})

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("не удалось прочитать вывод: %v", err)
	}
	output := buf.String()

	if exitCode != ExitSuccess {
		t.Errorf("version должна возвращать exit code 0, получено %d", exitCode)
	}

	if !strings.Contains(output, "s3back") {
		t.Errorf("вывод version должен содержать 's3back': %q", output)
	}
}

// TestUnknownCommand проверяет, что неизвестная команда возвращает ExitArgError.
func TestUnknownCommand(t *testing.T) {
	// Перехватываем stderr
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	exitCode := Run([]string{"unknowncmd"})

	w.Close()
	os.Stderr = oldStderr

	if exitCode != ExitArgError {
		t.Errorf("неизвестная команда должна возвращать %d, получено %d", ExitArgError, exitCode)
	}
}
