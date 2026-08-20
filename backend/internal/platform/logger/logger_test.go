package logger

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	apptrace "github.com/thinhnguyenwilliam/book-store/backend/internal/platform/trace"
)

func TestDailyWriterCreatesOneFilePerDate(t *testing.T) {
	directory := t.TempDir()
	location := time.FixedZone("test", 7*60*60)
	current := time.Date(2026, time.August, 20, 23, 59, 0, 0, location)
	var clockMu sync.Mutex
	now := func() time.Time {
		clockMu.Lock()
		defer clockMu.Unlock()
		return current
	}
	w, err := newDailyWriter("userservice", testConfig(directory), location, now)
	if err != nil {
		t.Fatalf("newDailyWriter() error = %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	if _, err := w.Write([]byte("before midnight\n")); err != nil {
		t.Fatalf("Write() before midnight error = %v", err)
	}
	clockMu.Lock()
	current = current.Add(2 * time.Minute)
	clockMu.Unlock()
	if _, err := w.Write([]byte("after midnight\n")); err != nil {
		t.Fatalf("Write() after midnight error = %v", err)
	}

	for _, name := range []string{"userservice-2026-08-20.log", "userservice-2026-08-21.log"} {
		if _, err := os.Stat(filepath.Join(directory, name)); err != nil {
			t.Fatalf("expected log file %q: %v", name, err)
		}
	}
}

func TestNextMidnightUsesConfiguredLocation(t *testing.T) {
	location := time.FixedZone("test", 7*60*60)
	now := time.Date(2026, time.August, 20, 19, 30, 0, 0, location)
	want := time.Date(2026, time.August, 21, 0, 0, 0, 0, location)
	if got := nextMidnight(now); !got.Equal(want) {
		t.Fatalf("nextMidnight() = %s, want %s", got, want)
	}
}

func TestLoggerAddsTraceIDFromContext(t *testing.T) {
	directory := t.TempDir()
	manager, err := New("bookservice", Config{
		Directory:  directory,
		Level:      "info",
		Format:     "json",
		TimeZone:   "Asia/Ho_Chi_Minh",
		MaxSizeMB:  100,
		MaxAgeDays: 14,
		MaxBackups: 30,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	const traceID = "0123456789abcdef0123456789abcdef"
	ctx := apptrace.ContextWithID(context.Background(), traceID)
	manager.Logger().InfoContext(ctx, "traced message")
	if err := manager.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	files, err := filepath.Glob(filepath.Join(directory, "bookservice-*.log"))
	if err != nil || len(files) != 1 {
		t.Fatalf("log files = %v, error = %v", files, err)
	}
	content, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(content), `"trace_id":"`+traceID+`"`) {
		t.Fatalf("log does not contain trace ID: %s", content)
	}
}

func TestTextTimeFormatterUsesHumanReadableLocalTime(t *testing.T) {
	location := time.FixedZone("Vietnam", 7*60*60)
	formatter := textTimeFormatter(location)
	timestamp := time.Date(2026, time.August, 20, 9, 10, 30, 189_000_000, time.UTC)
	formatted := formatter(nil, slog.Time(slog.TimeKey, timestamp))

	if got, want := formatted.Value.String(), "20/08/2026 16:10:30.189 +07:00"; got != want {
		t.Fatalf("formatted time = %q, want %q", got, want)
	}
}

func TestDailyWriterRotatesBySizeAndLimitsBackups(t *testing.T) {
	directory := t.TempDir()
	location := time.FixedZone("test", 7*60*60)
	now := func() time.Time {
		return time.Date(2026, time.August, 20, 12, 0, 0, 0, location)
	}
	cfg := testConfig(directory)
	cfg.MaxSizeMB = 1
	cfg.MaxBackups = 1
	w, err := newDailyWriter("bookservice", cfg, location, now)
	if err != nil {
		t.Fatalf("newDailyWriter() error = %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	payload := bytes.Repeat([]byte("a"), 700*1024)
	for index := 0; index < 3; index++ {
		if _, err := w.Write(payload); err != nil {
			t.Fatalf("Write() #%d error = %v", index+1, err)
		}
	}

	activePath := filepath.Join(directory, "bookservice-2026-08-20.log")
	latestBackup := filepath.Join(directory, "bookservice-2026-08-20.002.log")
	removedBackup := filepath.Join(directory, "bookservice-2026-08-20.001.log")
	for _, path := range []string{activePath, latestBackup} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected log file %q: %v", path, err)
		}
	}
	if _, err := os.Stat(removedBackup); !os.IsNotExist(err) {
		t.Fatalf("old size backup should be removed, stat error = %v", err)
	}
}

func TestDailyWriterRemovesExpiredServiceLogs(t *testing.T) {
	directory := t.TempDir()
	location := time.FixedZone("test", 7*60*60)
	current := time.Date(2026, time.August, 20, 12, 0, 0, 0, location)

	oldPath := filepath.Join(directory, "userservice-2026-08-01.log")
	recentPath := filepath.Join(directory, "userservice-2026-08-19.log")
	unrelatedPath := filepath.Join(directory, "bookservice-2026-08-01.log")
	for _, path := range []string{oldPath, recentPath, unrelatedPath} {
		if err := os.WriteFile(path, []byte("log"), 0o640); err != nil {
			t.Fatalf("write fixture %q: %v", path, err)
		}
	}
	oldTime := time.Date(2026, time.August, 1, 12, 0, 0, 0, location)
	recentTime := time.Date(2026, time.August, 19, 12, 0, 0, 0, location)
	if err := os.Chtimes(oldPath, oldTime, oldTime); err != nil {
		t.Fatalf("age old fixture: %v", err)
	}
	if err := os.Chtimes(recentPath, recentTime, recentTime); err != nil {
		t.Fatalf("age recent fixture: %v", err)
	}
	if err := os.Chtimes(unrelatedPath, oldTime, oldTime); err != nil {
		t.Fatalf("age unrelated fixture: %v", err)
	}

	w, err := newDailyWriter("userservice", testConfig(directory), location, func() time.Time { return current })
	if err != nil {
		t.Fatalf("newDailyWriter() error = %v", err)
	}
	t.Cleanup(func() { _ = w.Close() })

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expired service log should be removed, stat error = %v", err)
	}
	for _, path := range []string{recentPath, unrelatedPath} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("log file %q should remain: %v", path, err)
		}
	}
}

func testConfig(directory string) Config {
	return Config{
		Directory:  directory,
		Level:      "info",
		Format:     "json",
		TimeZone:   "Asia/Ho_Chi_Minh",
		MaxSizeMB:  100,
		MaxAgeDays: 14,
		MaxBackups: 30,
	}
}
