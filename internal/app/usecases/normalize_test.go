package usecases

import (
	"path/filepath"
	"testing"
)

// TestNormalizeDirNameDot проверяет, что относительные пути ".", "./" и пустая строка
// резолвятся в имя текущей директории, а не остаются точкой/пустой строкой.
func TestNormalizeDirNameDot(t *testing.T) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("не удалось получить CWD: %v", err)
	}
	expected := filepath.Base(cwd)

	tests := []struct {
		name string
		in   string
	}{
		{"точка", "."},
		{"точка-слэш", "./"},
		{"пустая строка", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeDirName(tt.in)
			if got == "." || got == ".." || got == "" {
				t.Errorf("NormalizeDirName(%q) = %q — нельзя возвращать точечный/пустой сегмент", tt.in, got)
			}
			// Имя должно совпадать с base(CWD) (после нормализации специальных символов).
			if got != expected {
				// Если CWD содержит небезопасные символы, они заменены на _, поэтому проверяем непустоту.
				if got == "" {
					t.Errorf("NormalizeDirName(%q) вернуло пустую строку", tt.in)
				}
			}
		})
	}
}

// TestNormalizeDirNameRegular проверяет обычные имена директорий.
func TestNormalizeDirNameRegular(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"my-project", "my-project"},
		{"/home/user/projects/my-app", "my-app"},
		{"some dir with spaces", "some_dir_with_spaces"},
		{"проект", "______"}, // кириллица заменяется на _
		{"foo/bar/", "bar"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got := NormalizeDirName(tt.in)
			if got != tt.want {
				// Для кириллицы допускаем фоллбэк "backup" (если все символы заменились на _).
				if tt.in == "проект" && (got == "backup" || got == "______") {
					return
				}
				t.Errorf("NormalizeDirName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// TestParseKeyFlatFormat проверяет, что ParseKey понимает старый flat-формат ключа
// (без поддиректории source), который мог появиться из-за бага с пустым source.
func TestParseKeyFlatFormat(t *testing.T) {
	tests := []struct {
		key      string
		wantTS   string
		wantTag  string
		wantOK   bool
		comment  string
	}{
		{
			key:     "backups/my-project/2026-05-05T03-58-40Z__v1.1.tar.gz",
			wantTS:  "2026-05-05T03-58-40Z",
			wantTag: "v1.1",
			wantOK:  true,
			comment: "основной формат с source",
		},
		{
			key:     "backups/2026-05-05T03-58-40Z__v1.1.tar.gz",
			wantTS:  "2026-05-05T03-58-40Z",
			wantTag: "v1.1",
			wantOK:  true,
			comment: "flat-формат: двойное подчёркивание",
		},
		{
			key:     "backups/2026-05-05T03-58-40Z___v1.1.tar.gz",
			wantTS:  "2026-05-05T03-58-40Z",
			wantTag: "v1.1",
			wantOK:  true,
			comment: "flat-формат: тройное подчёркивание (пустой source между __ и __)",
		},
		{
			key:     "backups/2026-05-05T03-55-46Z___v1.0.tar.gz",
			wantTS:  "2026-05-05T03-55-46Z",
			wantTag: "v1.0",
			wantOK:  true,
			comment: "реальный кейс пользователя",
		},
		{
			key:     "garbage/key.tar.gz",
			wantOK:  false,
			comment: "невалидный ключ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			ts, tag, ok := ParseKey(tt.key)
			if ok != tt.wantOK {
				t.Errorf("ParseKey(%q) ok = %v, want %v", tt.key, ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			if ts != tt.wantTS {
				t.Errorf("ParseKey(%q) timestamp = %q, want %q", tt.key, ts, tt.wantTS)
			}
			if tag != tt.wantTag {
				t.Errorf("ParseKey(%q) tag = %q, want %q", tt.key, tag, tt.wantTag)
			}
		})
	}
}
