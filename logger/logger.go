package logger

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Logger struct {
	infoLog  *log.Logger
	errorLog *log.Logger
}

var Log *Logger

func InitLogger() {
	infoLog := log.New(os.Stdout, "\033[36m[INFO]\033[0m ", log.Ldate|log.Ltime)
	errorLog := log.New(os.Stderr, "\033[31m[ERROR]\033[0m ", log.Ldate|log.Ltime|log.Lshortfile)

	Log = &Logger{
		infoLog:  infoLog,
		errorLog: errorLog,
	}
}

func Info(format string, v ...interface{}) {
	if Log != nil && Log.infoLog != nil {
		Log.infoLog.Output(2, fmt.Sprintf(format, v...))
	} else {
		log.Printf("[INFO] "+format, v...)
	}
}

func Error(format string, v ...interface{}) {
	if Log != nil && Log.errorLog != nil {
		Log.errorLog.Output(2, fmt.Sprintf(format, v...))
	} else {
		log.Printf("[ERROR] "+format, v...)
	}
}

func LogRequest(method, path, ip string, status int, latency time.Duration, err string) {
	if status >= 400 {
		Error("HTTP %s %s | IP: %s | Status: %d | Latency: %v | Error: %s", method, path, ip, status, latency, err)
	} else {
		Info("HTTP %s %s | IP: %s | Status: %d | Latency: %v", method, path, ip, status, latency)
	}
}
