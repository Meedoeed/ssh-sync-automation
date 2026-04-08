package logger

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

var SuperLogger *zerolog.Logger

func Init(level string, pretty bool) { // функция-конструктор для глобального логгера SuperLogger
	zerolog.TimeFieldFormat = time.RFC3339Nano

	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	var logger zerolog.Logger

	if pretty { // для отладки человеко-читаемый
		output := zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: "15:04:05.000",
			NoColor:    false,
		}
		logger = zerolog.New(output).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(os.Stdout).With().Timestamp().Logger()
	}

	SuperLogger = &logger
	log.Logger = logger // глобальный логгер поменял на настроенный с дефолтного
}

func Get() *zerolog.Logger { // геттер для логгера
	if SuperLogger == nil {
		Init("Info", true) // если не инициализировали логгер ранее - создаём дефолтный отладочный
	}
	return SuperLogger
}
