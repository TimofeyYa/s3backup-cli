package cli

import (
	"reflect"
	"testing"
)

// TestReorderArgs проверяет переупорядочивание аргументов перед flag.Parse.
func TestReorderArgs(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "флаги после позиционного аргумента",
			in:   []string{"./", "-tag", "v1", "-to", "bucket"},
			want: []string{"-tag", "v1", "-to", "bucket", "./"},
		},
		{
			name: "флаги до позиционного аргумента",
			in:   []string{"-tag", "v1", "-to", "bucket", "./"},
			want: []string{"-tag", "v1", "-to", "bucket", "./"},
		},
		{
			name: "форма flag=value",
			in:   []string{"./", "-tag=v1", "-to=bucket"},
			want: []string{"-tag=v1", "-to=bucket", "./"},
		},
		{
			name: "двойной дефис",
			in:   []string{"./", "--tag", "v1", "--force"},
			want: []string{"--tag", "v1", "--force", "./"},
		},
		{
			name: "булев флаг force без значения",
			in:   []string{"./restore", "-from", "bucket", "--force"},
			want: []string{"-from", "bucket", "--force", "./restore"},
		},
		{
			name: "булев флаг dry-run без значения",
			in:   []string{"-from", "bucket", "--dry-run"},
			want: []string{"-from", "bucket", "--dry-run"},
		},
		{
			name: "только позиционные",
			in:   []string{"./"},
			want: []string{"./"},
		},
		{
			name: "пустой ввод",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reorderArgs(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("reorderArgs(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
