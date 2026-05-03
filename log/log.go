package log

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"

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
	levelVar      = new(slog.LevelVar)
	fileWriter    *lumberjack.Logger
)

// Init 初始化全局 logger
func Init(cfg Config) {
	// 默认配置
	if cfg.LogPath == "" {
		cfg = Config{
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
	level := parseLogLevel(cfg.Level)

	// 2. 构建输出 writer
	var writers []io.Writer
	if cfg.Console {
		writers = append(writers, os.Stderr)
	}
	if cfg.LogPath != "" {
		fileWriter = &lumberjack.Logger{
			Filename:   cfg.LogPath,
			MaxSize:    cfg.MaxSize,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,
			Compress:   cfg.Compress,
			LocalTime:  true,
		}
		writers = append(writers, fileWriter)
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stderr)
	}

	multiWriter := io.MultiWriter(writers...)

	// 3. 设置初始日志级别
	levelVar.Set(level)

	// 4. 处理器选项
	opts := &slog.HandlerOptions{
		AddSource:   true,
		Level:       levelVar,
		ReplaceAttr: replaceSourceAttr,
	}

	// 4. 创建 handler
	var handler slog.Handler
	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(multiWriter, opts)
	} else {
		handler = slog.NewTextHandler(multiWriter, opts)
	}

	// 5. 设置全局 logger
	defaultLogger = slog.New(handler)
	slog.SetDefault(defaultLogger)
}

// parseLogLevel 解析日志级别字符串
func parseLogLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// replaceSourceAttr 只保留文件名，不显示全路径
func replaceSourceAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key != slog.SourceKey {
		return a
	}

	source, ok := a.Value.Any().(*slog.Source)
	if !ok || source == nil {
		return a
	}

	// 官方标准方法获取文件名
	source.File = filepath.Base(source.File)
	return slog.Any(slog.SourceKey, source)
}

// ----- 对外公开 API -----

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

// Fatal 打印日志并退出程序 exit(1)
func Fatal(msg string, args ...any) {
	defaultLogger.Log(context.Background(), levelFatal, msg, args...)
	_ = Sync() // 退出前刷盘
	os.Exit(1)
}

// Panic 打印日志并触发 panic
func Panic(msg string, args ...any) {
	defaultLogger.Log(context.Background(), levelPanic, msg, args...)
	_ = Sync()
	panic(msg)
}

// With 携带固定字段的子 logger
func With(args ...any) *slog.Logger {
	return defaultLogger.With(args...)
}

// Sync 刷新日志缓冲区（程序退出前必须调用）
func Sync() error {
	if fileWriter != nil {
		return fileWriter.Close()
	}
	return nil
}

// SetLevel 动态修改日志级别
func SetLevel(level string) {
	levelVar.Set(parseLogLevel(level))
}

// GetLevel 获取当前日志级别
func GetLevel() slog.Level {
	return levelVar.Level()
}
