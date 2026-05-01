package log

import (
	"context"
	"io"
	"log/slog"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Config 日志配置
type Config struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
	LogPath    string // 日志文件路径，空表示不输出到文件
	MaxSize    int    // 单个日志文件最大大小（MB），默认100
	MaxBackups int    // 保留的旧日志文件最大数量，默认7
	MaxAge     int    // 保留旧日志文件的最大天数，默认30 days
	Compress   bool   // 是否压缩旧日志，默认false
	Console    bool   // 是否同时输出到控制台，默认true
}

const (
	levelFatal = slog.Level(12)
	levelPanic = slog.Level(14)
)

// 全局 logger 实例
var (
	defaultLogger *slog.Logger
	fileWriter    *lumberjack.Logger
)

// Init 初始化全局 logger
func Init(cfg *Config) error {
	if cfg == nil {
		cfg = &Config{
			Level:      "info",
			Format:     "text",
			LogPath:    "",
			MaxSize:    100,
			MaxBackups: 7,
			MaxAge:     30,
			Compress:   false,
			Console:    true,
		}
	}

	// 1. 解析日志级别
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// 2. 构建输出 writer（可能多个：文件 + 控制台）
	var writers []io.Writer

	if cfg.Console {
		writers = append(writers, os.Stderr)
	}
	if cfg.LogPath != "" {
		// 使用 lumberjack 实现自动切割
		fileWriter = &lumberjack.Logger{
			Filename:   cfg.LogPath,
			MaxSize:    cfg.MaxSize, // MB
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge, // days
			Compress:   cfg.Compress,
			LocalTime:  true,
		}
		writers = append(writers, fileWriter)
	}

	// 如果没有配置任何输出，默认输出到 stderr
	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	multiWriter := io.MultiWriter(writers...)

	// 3. 选择格式：JSON 或 Text
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		AddSource: true, // 显示调用文件和行号
		Level:     level,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// 自定义某些字段名（可选）
			if a.Key == slog.SourceKey {

				// 只保留文件名和行号，不要完整路径
				if source, ok := a.Value.Any().(*slog.Source); ok && source != nil {
					_, file := splitPath(source.File)
					source.File = file
					a.Value = slog.AnyValue(source)
				}
			}
			return a
		},
	}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(multiWriter, opts)
	} else {
		handler = slog.NewTextHandler(multiWriter, opts)
	}

	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger) // 同时覆盖标准库的全局 logger
	return nil
}

// splitPath 分割路径获取文件名部分
func splitPath(path string) (dir, file string) {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i], path[i+1:]
		}
	}
	return "", path
}

// ----- 对外公开 API（直接使用 slog 方法）-----
// 也可以保留简单函数，内部调用 defaultLogger

func Debug(msg string, args ...any) {
	defaultLogger.Debug(msg, args...)
}
func Info(msg string, args ...any) {
	defaultLogger.Info(msg, args...)
}
func Warn(msg string, args ...any) {
	defaultLogger.Warn(msg, args...)
}
func Error(msg string, args ...any) {
	defaultLogger.Error(msg, args...)
}
func Fatal(msg string, args ...any) {
	defaultLogger.Log(context.Background(), levelFatal, msg, args...)
	os.Exit(1)
}
func Panic(msg string, args ...any) {
	defer panic(msg)
	defaultLogger.Log(context.Background(), levelPanic, msg, args...)
}

// With 返回带有预设属性的子 logger
func With(args ...any) *slog.Logger {
	return defaultLogger.With(args...)
}

// Sync 刷新缓冲区（对于文件输出很重要）
func Sync() error {
	if fileWriter != nil {
		return fileWriter.Close()
	}
	return nil
}
