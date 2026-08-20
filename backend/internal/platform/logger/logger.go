package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
}

// Manager owns the active daily log file and its midnight rotation worker.
type Manager struct {
	logger *slog.Logger
	writer *dailyWriter
}

// New creates a service logger whose file is named service-YYYY-MM-DD.log.
func New(service string, cfg Config) (*Manager, error) {
	location, err := time.LoadLocation(cfg.TimeZone)
	if err != nil {
		return nil, fmt.Errorf("load logging timezone %q: %w", cfg.TimeZone, err)
	}

	level, err := parseLevel(cfg.Level)
	if err != nil {
		return nil, err
	}

	writer, err := newDailyWriter(service, cfg.Directory, location, time.Now)
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
	mu        sync.Mutex
	service   string
	directory string
	location  *time.Location
	now       func() time.Time
	file      *os.File
	date      string
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}

func newDailyWriter(service, directory string, location *time.Location, now func() time.Time) (*dailyWriter, error) {
	if strings.TrimSpace(service) == "" || strings.ContainsAny(service, `/\\`) {
		return nil, fmt.Errorf("logging service name is invalid")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create log directory %q: %w", directory, err)
	}

	w := &dailyWriter{
		service:   service,
		directory: directory,
		location:  location,
		now:       now,
		done:      make(chan struct{}),
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
	if now.Format(time.DateOnly) != w.date {
		if err := w.rotateLocked(now); err != nil {
			return 0, err
		}
	}
	return w.file.Write(data)
}

func (w *dailyWriter) rotationLoop(ctx context.Context) {
	defer close(w.done)
	for {
		now := w.now().In(w.location)
		next := nextMidnight(now)
		timer := time.NewTimer(time.Until(next))
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

	path := filepath.Join(w.directory, fmt.Sprintf("%s-%s.log", w.service, date))
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", path, err)
	}
	oldFile := w.file
	w.file = file
	w.date = date
	if oldFile != nil {
		if err := oldFile.Close(); err != nil {
			return fmt.Errorf("close previous log file: %w", err)
		}
	}
	return nil
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
