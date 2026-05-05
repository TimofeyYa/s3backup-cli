// Точка входа CLI-приложения s3back.
// s3back — утилита для резервного копирования директорий в S3-совместимое хранилище.
package main

import (
	"os"

	"github.com/sypertimka/s3back/internal/cli"
)

// Version — версия приложения. Переопределяется через ldflags при сборке:
//
//	go build -ldflags "-X main.Version=0.1.0" ./cmd/s3back
var Version = "0.1.0"

func main() {
	// Передаём версию в CLI пакет
	cli.Version = Version

	// Запускаем CLI с аргументами командной строки (без имени программы)
	exitCode := cli.Run(os.Args[1:])
	os.Exit(exitCode)
}
