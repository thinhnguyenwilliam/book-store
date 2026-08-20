package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // Include the IANA timezone database in every service binary.
)

// Config controls the structured application logger shared by every service.
type Config struct {
	Directory  string `mapstructure:"directory"`
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	TimeZone   string `mapstructure:"timezone"`
	AlsoStdout bool   `mapstructure:"also_stdout"`
	MaxSizeMB  int    `mapstructure:"max_size_mb"`
	MaxAgeDays int    `mapstructure:"max_age_days"`
	MaxBackups int    `mapstructure:"max_backups"`
}

// Manager owns the active daily log file and its midnight rotation worker.
type Manager struct {
	logger *slog.Logger
	writer *dailyWriter
}

// New creates a service logger whose file is named service-YYYY-MM-DD.log.
func New(service string, cfg Config) (*Manager, error) {
	if cfg.MaxSizeMB < 1 {
		return nil, fmt.Errorf("logging.max_size_mb must be greater than zero")
	}
	if cfg.MaxAgeDays < 0 {
		return nil, fmt.Errorf("logging.max_age_days must not be negative")
	}
	if cfg.MaxBackups < 0 {
		return nil, fmt.Errorf("logging.max_backups must not be negative")
	}

	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("load logging timezone %q: %w", cfg.TimeZone, err)
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	writer, err := newDailyWriter(service, cfg, location, time.Now)
	if err != nil {
		return nil, err
	}

	var output io.Writer = writer
	if cfg.AlsoStdout {
		output = io.MultiWriter(os.Stdout, writer)
	}

	handlerOptions := &slog.HandlerOptions{Level: level, AddSource: true}
	var handler slog.Handler
	switch strings.ToLower(cfg.Format) {
	case "json":
		handler = slog.NewJSONHandler(output, handlerOptions)
	case "text":
		handlerOptions.ReplaceAttr = textTimeFormatter(location)
		handler = slog.NewTextHandler(output, handlerOptions)
	default:
		_ = writer.Close()
		return nil, fmt.Errorf("logging.format must be json or text")
	}
	handler = contextHandler{next: handler}

	return &Manager{
		logger: slog.New(handler).With("service", service),
		writer: writer,
	}, nil
}

func textTimeFormatter(location *time.Location) func([]string, slog.Attr) slog.Attr {
	return func(groups []string, attribute slog.Attr) slog.Attr {
		if len(groups) == 0 && attribute.Key == slog.TimeKey {
			if timestamp, ok := attribute.Value.Any().(time.Time); ok {
				attribute.Value = slog.StringValue(timestamp.In(location).Format("02/01/2006 15:04:05.000 -07:00"))
			}
		}
		return attribute
	}
}

func (m *Manager) Logger() *slog.Logger {
	return m.logger
}

func (m *Manager) Close() error {
	return m.writer.Close()
}

func parseLevel(value string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(strings.ToLower(value))); err != nil {
		return 0, fmt.Errorf("invalid logging.level %q: %w", value, err)
	}
	return level, nil
}

type dailyWriter struct {
	mu           sync.Mutex
	service      string
	directory    string
	location     *time.Location
	now          func() time.Time
	maxSizeBytes int64
	maxAgeDays   int
	maxBackups   int
	file         *os.File
	date         string
	size         int64
	cancel       context.CancelFunc
	done         chan struct{}
	closeOnce    sync.Once
}

func newDailyWriter(service string, cfg Config, location *time.Location, now func() time.Time) (*dailyWriter, error) {
	if strings.TrimSpace(service) == "" || strings.ContainsAny(service, `/\\`) {
		return nil, fmt.Errorf("logging service name is invalid")
	}
	if err := os.MkdirAll(cfg.Directory, 0o750); err != nil {
		return nil, fmt.Errorf("create log directory %q: %w", cfg.Directory, err)
	}

	w := &dailyWriter{
		service:      service,
		directory:    cfg.Directory,
		location:     location,
		now:          now,
		maxSizeBytes: int64(cfg.MaxSizeMB) * 1024 * 1024,
		maxAgeDays:   cfg.MaxAgeDays,
		maxBackups:   cfg.MaxBackups,
		done:         make(chan struct{}),
	}
	if err := w.rotate(now().In(location)); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	go w.rotationLoop(ctx)
	return w, nil
}

func (w *dailyWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now().In(w.location)
	if w.file == nil || now.Format(time.DateOnly) != w.date {
		if err := w.rotateLocked(now); err != nil {
			return 0, err
		}
	}
	if w.size > 0 && w.size+int64(len(data)) > w.maxSizeBytes {
		if err := w.rotateSizeLocked(now); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(data)
	w.size += int64(n)
	return n, err
}

func (w *dailyWriter) rotationLoop(ctx context.Context) {
	defer close(w.done)
	for {
		now := w.now().In(w.location)
		next := nextMidnight(now)
		timer := time.NewTimer(next.Sub(now))
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case rotationTime := <-timer.C:
			w.mu.Lock()
			if err := w.rotateLocked(rotationTime.In(w.location)); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "rotate log file for %s: %v\n", w.service, err)
			}
			w.mu.Unlock()
		}
	}
}

func nextMidnight(now time.Time) time.Time {
	year, month, day := now.Date()
	return time.Date(year, month, day+1, 0, 0, 0, 0, now.Location())
}

func (w *dailyWriter) rotate(now time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.rotateLocked(now)
}

func (w *dailyWriter) rotateLocked(now time.Time) error {
	date := now.Format(time.DateOnly)
	if w.file != nil && w.date == date {
		return nil
	}

	path := w.activePath(date)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", path, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("stat log file %q: %w", path, err)
	}
	oldFile := w.file
	w.file = file
	w.date = date
	w.size = info.Size()
	if oldFile != nil {
		if err := oldFile.Close(); err != nil {
			return fmt.Errorf("close previous log file: %w", err)
		}
	}
	w.reportCleanupError(w.cleanupLocked(now, path))
	return nil
}

func (w *dailyWriter) rotateSizeLocked(now time.Time) error {
	activePath := w.activePath(w.date)
	archivePath, err := w.nextArchivePath(w.date)
	if err != nil {
		return err
	}
	if err := w.file.Close(); err != nil {
		return fmt.Errorf("close full log file %q: %w", activePath, err)
	}
	w.file = nil
	if err := os.Rename(activePath, archivePath); err != nil {
		return fmt.Errorf("archive full log file %q: %w", activePath, err)
	}

	file, err := os.OpenFile(activePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		if restoreErr := os.Rename(archivePath, activePath); restoreErr == nil {
			w.reopenActiveAfterFailedRotation(activePath)
		}
		return fmt.Errorf("open new log file %q: %w", activePath, err)
	}
	w.file = file
	w.size = 0
	w.reportCleanupError(w.cleanupLocked(now, activePath))
	return nil
}

func (w *dailyWriter) reopenActiveAfterFailedRotation(activePath string) {
	file, err := os.OpenFile(activePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	w.file = file
	if info, statErr := file.Stat(); statErr == nil {
		w.size = info.Size()
	}
}

func (w *dailyWriter) activePath(date string) string {
	return filepath.Join(w.directory, fmt.Sprintf("%s-%s.log", w.service, date))
}

func (w *dailyWriter) nextArchivePath(date string) (string, error) {
	entries, err := os.ReadDir(w.directory)
	if err != nil {
		return "", fmt.Errorf("read log directory for next archive: %w", err)
	}
	prefix := fmt.Sprintf("%s-%s.", w.service, date)
	maxSequence := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
			continue
		}
		sequenceText := strings.TrimSuffix(strings.TrimPrefix(name, prefix), ".log")
		sequence, parseErr := strconv.Atoi(sequenceText)
		if parseErr == nil && sequence > maxSequence {
			maxSequence = sequence
		}
	}
	if maxSequence >= 999999 {
		return "", fmt.Errorf("too many log archives for %s", date)
	}
	return filepath.Join(
		w.directory,
		fmt.Sprintf("%s-%s.%03d.log", w.service, date, maxSequence+1),
	), nil
}

type backupFile struct {
	path    string
	name    string
	modTime time.Time
}

func (w *dailyWriter) cleanupLocked(now time.Time, activePath string) error {
	entries, err := os.ReadDir(w.directory)
	if err != nil {
		return fmt.Errorf("read log directory: %w", err)
	}

	cutoff := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, w.location).
		AddDate(0, 0, -w.maxAgeDays)
	backups := make([]backupFile, 0, len(entries))
	var cleanupErr error
	for _, entry := range entries {
		if entry.IsDir() || !w.managesFile(entry.Name()) {
			continue
		}
		path := filepath.Join(w.directory, entry.Name())
		if path == activePath {
			continue
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("stat old log %q: %w", path, infoErr))
			continue
		}
		if w.maxAgeDays > 0 && info.ModTime().Before(cutoff) {
			if removeErr := os.Remove(path); removeErr != nil {
				cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove expired log %q: %w", path, removeErr))
			}
			continue
		}
		backups = append(backups, backupFile{path: path, name: entry.Name(), modTime: info.ModTime()})
	}

	if w.maxBackups == 0 || len(backups) <= w.maxBackups {
		return cleanupErr
	}
	sort.Slice(backups, func(i, j int) bool {
		if backups[i].modTime.Equal(backups[j].modTime) {
			return backups[i].name > backups[j].name
		}
		return backups[i].modTime.After(backups[j].modTime)
	})
	for _, backup := range backups[w.maxBackups:] {
		if removeErr := os.Remove(backup.path); removeErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove excess log %q: %w", backup.path, removeErr))
		}
	}
	return cleanupErr
}

func (w *dailyWriter) managesFile(name string) bool {
	prefix := w.service + "-"
	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".log") {
		return false
	}
	remainder := strings.TrimPrefix(name, prefix)
	if len(remainder) < len(time.DateOnly)+len(".log") {
		return false
	}
	if _, err := time.Parse(time.DateOnly, remainder[:len(time.DateOnly)]); err != nil {
		return false
	}
	suffix := remainder[len(time.DateOnly):]
	if suffix == ".log" {
		return true
	}
	if !strings.HasPrefix(suffix, ".") || !strings.HasSuffix(suffix, ".log") {
		return false
	}
	sequence := strings.TrimSuffix(strings.TrimPrefix(suffix, "."), ".log")
	number, err := strconv.Atoi(sequence)
	return err == nil && number > 0
}

func (w *dailyWriter) reportCleanupError(err error) {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "clean old log files for %s: %v\n", w.service, err)
	}
}

func (w *dailyWriter) Close() error {
	var closeErr error
	w.closeOnce.Do(func() {
		w.cancel()
		<-w.done
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.file != nil {
			closeErr = w.file.Close()
			w.file = nil
		}
	})
	return closeErr
}
