package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// LogLevel represents the severity of a log entry
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

// Event types for machine-readable logging
const (
	EventDetectionComplete = "detection_complete"
	EventPlanGenerated    = "plan_generated"
	EventBuildStart       = "build_start"
	EventBuildComplete    = "build_complete"
	EventCacheHit         = "cache_hit"
	EventCacheMiss        = "cache_miss"
	EventOptimization     = "optimization_applied"
	EventError            = "error"
)

// MachineLog represents a structured log entry
type MachineLog struct {
	Event          string                 `json:"event"`
	Timestamp      string                 `json:"timestamp"`
	DurationMs     int64                  `json:"duration_ms,omitempty"`
	Language       string                 `json:"language,omitempty"`
	Framework      string                 `json:"framework,omitempty"`
	Strategy       string                 `json:"strategy,omitempty"`
	CacheHitRate   float64               `json:"cache_hit_rate,omitempty"`
	Confidence     float64                `json:"confidence,omitempty"`
	ImageSizeMB    int                    `json:"image_size_mb,omitempty"`
	Message        string                 `json:"message,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// Logger handles both human and machine-readable logging
type Logger struct {
	mu          sync.Mutex
	humanMode   bool
	output      *os.File
	machineLogs []MachineLog
	startTime   time.Time
}

// NewLogger creates a new logger
func NewLogger(humanMode bool) *Logger {
	return &Logger{
		humanMode:   humanMode,
		output:      os.Stdout,
		machineLogs: make([]MachineLog, 0),
		startTime:   time.Now(),
	}
}

// Log outputs a log entry
func (l *Logger) Log(level LogLevel, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05.000")

	var prefix string
	switch level {
	case DEBUG:
		prefix = "⚙️  DEBUG"
	case INFO:
		prefix = "ℹ️  INFO"
	case WARN:
		prefix = "⚠️  WARN"
	case ERROR:
		prefix = "❌ ERROR"
	}

	if l.humanMode {
		fmt.Fprintf(l.output, "[%s] %s %s\n", timestamp, prefix, msg)
	} else {
		fmt.Fprintf(l.output, "  %s\n", msg)
	}
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.Log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.Log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.Log(ERROR, format, args...)
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.Log(DEBUG, format, args...)
}

// DetectionComplete logs detection results
func (l *Logger) DetectionComplete(language, framework string, confidence float64, duration time.Duration) {
	ml := MachineLog{
		Event:       EventDetectionComplete,
		Timestamp:    time.Now().Format(time.RFC3339),
		DurationMs:   duration.Milliseconds(),
		Language:     language,
		Framework:    framework,
		Confidence:   confidence,
	}

	if l.humanMode {
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		l.Info("  🔍 Detection Results")
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		l.Info("  Language:     %s", language)
		l.Info("  Framework:    %s", framework)
		l.Info("  Confidence:  %.2f%%", confidence*100)
		l.Info("  Duration:    %dms", duration.Milliseconds())
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	l.machineLogs = append(l.machineLogs, ml)
}

// PlanGenerated logs build plan generation
func (l *Logger) PlanGenerated(strategy string, stages int, optimizations []string, duration time.Duration) {
	ml := MachineLog{
		Event:         EventPlanGenerated,
		Timestamp:     time.Now().Format(time.RFC3339),
		DurationMs:    duration.Milliseconds(),
		Strategy:      strategy,
		Metadata:      map[string]interface{}{"stages": stages, "optimizations": optimizations},
	}

	if l.humanMode {
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		l.Info("  📋 Build Plan Generated")
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		l.Info("  Strategy:    %s", strategy)
		l.Info("  Stages:     %d", stages)
		l.Info("  Optimizations Applied:")
		for _, opt := range optimizations {
			l.Info("    • %s", opt)
		}
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	l.machineLogs = append(l.machineLogs, ml)
}

// BuildComplete logs build completion
func (l *Logger) BuildComplete(duration time.Duration, cacheHitRate float64, imageSizeMB int) {
	ml := MachineLog{
		Event:        EventBuildComplete,
		Timestamp:    time.Now().Format(time.RFC3339),
		DurationMs:   duration.Milliseconds(),
		CacheHitRate: cacheHitRate,
		ImageSizeMB:  imageSizeMB,
	}

	totalDuration := time.Since(l.startTime)

	if l.humanMode {
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		l.Info("  ✅ Build Complete")
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		l.Info("  Total Duration:  %dms (%.2fs)", totalDuration.Milliseconds(), totalDuration.Seconds())
		l.Info("  Cache Hit Rate: %.1f%%", cacheHitRate*100)
		l.Info("  Image Size:     %dMB", imageSizeMB)
		l.Info("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	}

	l.machineLogs = append(l.machineLogs, ml)
}

// CacheAccess logs cache hit/miss
func (l *Logger) CacheAccess(cacheType string, hit bool, key string) {
	event := EventCacheMiss
	if hit {
		event = EventCacheHit
	}

	ml := MachineLog{
		Event:   event,
		Metadata: map[string]interface{}{"type": cacheType, "key": key},
	}

	if l.humanMode {
		status := "❌ MISS"
		if hit {
			status = "✅ HIT"
		}
		l.Debug("  Cache %s: %s (%s)", status, cacheType, key)
	}

	l.machineLogs = append(l.machineLogs, ml)
}

// Optimization logs an optimization applied
func (l *Logger) Optimization(optType, description string) {
	ml := MachineLog{
		Event:    EventOptimization,
		Metadata: map[string]interface{}{"type": optType, "description": description},
	}

	if l.humanMode {
		l.Info("  ⚡ %s: %s", optType, description)
	}

	l.machineLogs = append(l.machineLogs, ml)
}

// PrintSummary prints a human-readable summary
func (l *Logger) PrintSummary() {
	if !l.humanMode {
		return
	}

	totalDuration := time.Since(l.startTime)
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("  📊 Build Summary")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  Total Time:   %.2fs\n", totalDuration.Seconds())
	fmt.Printf("  Machine Logs: %d entries\n", len(l.machineLogs))
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// OutputMachineLogs outputs all machine-readable logs as JSON
func (l *Logger) OutputMachineLogs() ([]byte, error) {
	return json.MarshalIndent(map[string]interface{}{
		"logs":      l.machineLogs,
		"total":     len(l.machineLogs),
		"timestamp": time.Now().Format(time.RFC3339),
	}, "", "  ")
}

// Error logs an error and adds to machine logs
func (l *Logger) ErrorLog(err error, context string) {
	l.Error("%s: %v", context, err)

	ml := MachineLog{
		Event:    EventError,
		Metadata: map[string]interface{}{"error": err.Error(), "context": context},
	}
	l.machineLogs = append(l.machineLogs, ml)
}

// StageLog logs a build stage
func (l *Logger) StageLog(stageName string, message string) {
	l.Info("  📦 Stage: %s - %s", stageName, message)
}

// Separator prints a visual separator
func (l *Logger) Separator() {
	if l.humanMode {
		fmt.Println(strings.Repeat("─", 50))
	}
}

// Header prints a section header
func (l *Logger) Header(title string) {
	if l.humanMode {
		l.Separator()
		fmt.Printf("  %s\n", title)
		l.Separator()
	}
}
