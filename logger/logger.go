package logger

import (
	"fmt"
	"os"
	"time"
)

type Level string

const (
	LevelInfo    Level = "INFO"
	LevelError   Level = "ERROR"
	LevelSuccess Level = "SUCCESS"
	LevelWarn    Level = "WARN"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
)

var colors = map[Level]string{
	LevelInfo:    ColorBlue,
	LevelError:   ColorRed,
	LevelSuccess: ColorGreen,
	LevelWarn:    ColorYellow,
}

// RegisterLevel yangi log turini (yoki mavjudining rangini) ro'yxatdan o'tkazadi,
// masalan: logger.RegisterLevel("DEBUG", logger.ColorYellow)
func RegisterLevel(level Level, color string) {
	colors[level] = color
}

func write(level Level, message string) {
	color, ok := colors[level]
	if !ok {
		color = ColorReset
	}

	out := os.Stdout
	if level == LevelError {
		out = os.Stderr
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")

	fmt.Fprintf(out, "%s[%s] %s - %s%s\n", color, level, timestamp, message, ColorReset)
}

// Log ixtiyoriy (RegisterLevel bilan qo'shilgan) turdagi formatlangan xabarni chiqaradi.
func Log(level Level, format string, args ...any) {
	write(level, fmt.Sprintf(format, args...))
}

// Info tayyor matnni o'zgartirmasdan chiqaradi (format direktivalari e'tiborga olinmaydi).
func Info(args ...any) {
	write(LevelInfo, fmt.Sprint(args...))
}

// Infof formatlangan xabar chiqaradi, masalan: logger.Infof("port: %d", port).
func Infof(format string, args ...any) {
	Log(LevelInfo, format, args...)
}

func Error(args ...any) {
	write(LevelError, fmt.Sprint(args...))
}

func Errorf(format string, args ...any) {
	Log(LevelError, format, args...)
}

func Success(args ...any) {
	write(LevelSuccess, fmt.Sprint(args...))
}

func Successf(format string, args ...any) {
	Log(LevelSuccess, format, args...)
}

func Warn(args ...any) {
	write(LevelWarn, fmt.Sprint(args...))
}

func Warnf(format string, args ...any) {
	Log(LevelWarn, format, args...)
}
