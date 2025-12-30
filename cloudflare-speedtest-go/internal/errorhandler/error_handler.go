package errorhandler

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ErrorType represents different types of errors
type ErrorType string

const (
	ErrorTypeNetwork    ErrorType = "network"
	ErrorTypeTimeout    ErrorType = "timeout"
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeSystem     ErrorType = "system"
	ErrorTypeDataCenter ErrorType = "datacenter"
	ErrorTypeSpeedTest  ErrorType = "speedtest"
	ErrorTypeConfig     ErrorType = "config"
	ErrorTypeFileIO     ErrorType = "fileio"
)

// ErrorSeverity represents the severity level of an error
type ErrorSeverity string

const (
	SeverityLow      ErrorSeverity = "low"
	SeverityMedium   ErrorSeverity = "medium"
	SeverityHigh     ErrorSeverity = "high"
	SeverityCritical ErrorSeverity = "critical"
)

// ErrorInfo contains detailed information about an error
type ErrorInfo struct {
	Type       ErrorType     `json:"type"`
	Severity   ErrorSeverity `json:"severity"`
	Message    string        `json:"message"`
	Context    string        `json:"context"`
	Timestamp  time.Time     `json:"timestamp"`
	Retryable  bool          `json:"retryable"`
	RetryCount int           `json:"retry_count"`
	MaxRetries int           `json:"max_retries"`
	Component  string        `json:"component"`
	Operation  string        `json:"operation"`
	Details    interface{}   `json:"details,omitempty"`
}

// RetryPolicy defines retry behavior for different error types
type RetryPolicy struct {
	MaxRetries    int           `json:"max_retries"`
	InitialDelay  time.Duration `json:"initial_delay"`
	MaxDelay      time.Duration `json:"max_delay"`
	BackoffFactor float64       `json:"backoff_factor"`
	Jitter        bool          `json:"jitter"`
}

// ErrorHandler manages error handling, retries, and logging
type ErrorHandler struct {
	mu            sync.RWMutex
	retryPolicies map[ErrorType]*RetryPolicy
	errorStats    map[ErrorType]*ErrorStats
	logger        *StructuredLogger
	degraded      bool
	degradedUntil time.Time
}

// ErrorStats tracks statistics for each error type
type ErrorStats struct {
	TotalCount   int       `json:"total_count"`
	RetryCount   int       `json:"retry_count"`
	SuccessCount int       `json:"success_count"`
	LastOccurred time.Time `json:"last_occurred"`
	LastSuccess  time.Time `json:"last_success"`
}

// StructuredLogger provides structured logging capabilities
type StructuredLogger struct {
	mu     sync.Mutex
	logger *log.Logger
}

// New creates a new error handler with default policies
func New() *ErrorHandler {
	eh := &ErrorHandler{
		retryPolicies: make(map[ErrorType]*RetryPolicy),
		errorStats:    make(map[ErrorType]*ErrorStats),
		logger:        NewStructuredLogger(),
	}

	// Set default retry policies
	eh.setDefaultPolicies()

	return eh
}

// NewStructuredLogger creates a new structured logger
func NewStructuredLogger() *StructuredLogger {
	return &StructuredLogger{
		logger: log.New(log.Writer(), "", log.LstdFlags|log.Lshortfile),
	}
}

// setDefaultPolicies sets default retry policies for different error types
func (eh *ErrorHandler) setDefaultPolicies() {
	policies := map[ErrorType]*RetryPolicy{
		ErrorTypeNetwork: {
			MaxRetries:    3,
			InitialDelay:  1 * time.Second,
			MaxDelay:      10 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		ErrorTypeTimeout: {
			MaxRetries:    2,
			InitialDelay:  2 * time.Second,
			MaxDelay:      15 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
		ErrorTypeDataCenter: {
			MaxRetries:    1,
			InitialDelay:  500 * time.Millisecond,
			MaxDelay:      2 * time.Second,
			BackoffFactor: 1.5,
			Jitter:        false,
		},
		ErrorTypeSpeedTest: {
			MaxRetries:    2,
			InitialDelay:  1 * time.Second,
			MaxDelay:      5 * time.Second,
			BackoffFactor: 1.5,
			Jitter:        true,
		},
		ErrorTypeValidation: {
			MaxRetries:    0, // Don't retry validation errors
			InitialDelay:  0,
			MaxDelay:      0,
			BackoffFactor: 1.0,
			Jitter:        false,
		},
		ErrorTypeSystem: {
			MaxRetries:    1,
			InitialDelay:  1 * time.Second,
			MaxDelay:      3 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        false,
		},
		ErrorTypeConfig: {
			MaxRetries:    0, // Don't retry config errors
			InitialDelay:  0,
			MaxDelay:      0,
			BackoffFactor: 1.0,
			Jitter:        false,
		},
		ErrorTypeFileIO: {
			MaxRetries:    2,
			InitialDelay:  500 * time.Millisecond,
			MaxDelay:      2 * time.Second,
			BackoffFactor: 2.0,
			Jitter:        true,
		},
	}

	for errorType, policy := range policies {
		eh.retryPolicies[errorType] = policy
	}
}

// HandleError processes an error with retry logic and logging
func (eh *ErrorHandler) HandleError(ctx context.Context, errorInfo *ErrorInfo) error {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	// Update error statistics
	eh.updateErrorStats(errorInfo)

	// Log the error
	eh.logError(errorInfo)

	// Check if error is retryable
	if !errorInfo.Retryable {
		return fmt.Errorf("non-retryable error: %s", errorInfo.Message)
	}

	// Get retry policy for this error type
	policy, exists := eh.retryPolicies[errorInfo.Type]
	if !exists || policy.MaxRetries == 0 {
		return fmt.Errorf("no retry policy or retries exhausted: %s", errorInfo.Message)
	}

	// Check if we've exceeded max retries
	if errorInfo.RetryCount >= policy.MaxRetries {
		eh.logger.LogError("Max retries exceeded", map[string]interface{}{
			"error_type":  errorInfo.Type,
			"retry_count": errorInfo.RetryCount,
			"max_retries": policy.MaxRetries,
			"component":   errorInfo.Component,
			"operation":   errorInfo.Operation,
		})
		return fmt.Errorf("max retries (%d) exceeded: %s", policy.MaxRetries, errorInfo.Message)
	}

	// Calculate delay for next retry
	delay := eh.calculateRetryDelay(policy, errorInfo.RetryCount)

	eh.logger.LogInfo("Scheduling retry", map[string]interface{}{
		"error_type":  errorInfo.Type,
		"retry_count": errorInfo.RetryCount + 1,
		"delay_ms":    delay.Milliseconds(),
		"component":   errorInfo.Component,
		"operation":   errorInfo.Operation,
	})

	// Wait for retry delay
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		// Continue with retry
	}

	return nil // Indicate that retry should be attempted
}

// HandleSuccess records a successful operation
func (eh *ErrorHandler) HandleSuccess(errorType ErrorType, component, operation string) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	stats := eh.getOrCreateErrorStats(errorType)
	stats.SuccessCount++
	stats.LastSuccess = time.Now()

	eh.logger.LogInfo("Operation succeeded", map[string]interface{}{
		"error_type":    errorType,
		"component":     component,
		"operation":     operation,
		"success_count": stats.SuccessCount,
	})
}

// SetRetryPolicy sets a custom retry policy for an error type
func (eh *ErrorHandler) SetRetryPolicy(errorType ErrorType, policy *RetryPolicy) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	eh.retryPolicies[errorType] = policy
}

// GetRetryPolicy returns the retry policy for an error type
func (eh *ErrorHandler) GetRetryPolicy(errorType ErrorType) *RetryPolicy {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	if policy, exists := eh.retryPolicies[errorType]; exists {
		return policy
	}
	return nil
}

// GetErrorStats returns error statistics for all error types
func (eh *ErrorHandler) GetErrorStats() map[ErrorType]*ErrorStats {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	result := make(map[ErrorType]*ErrorStats)
	for errorType, stats := range eh.errorStats {
		result[errorType] = &ErrorStats{
			TotalCount:   stats.TotalCount,
			RetryCount:   stats.RetryCount,
			SuccessCount: stats.SuccessCount,
			LastOccurred: stats.LastOccurred,
			LastSuccess:  stats.LastSuccess,
		}
	}

	return result
}

// EnableDegradedMode enables degraded mode for a specified duration
func (eh *ErrorHandler) EnableDegradedMode(duration time.Duration) {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	eh.degraded = true
	eh.degradedUntil = time.Now().Add(duration)

	eh.logger.LogWarning("Degraded mode enabled", map[string]interface{}{
		"duration_minutes": duration.Minutes(),
		"until":            eh.degradedUntil.Format(time.RFC3339),
	})
}

// IsInDegradedMode returns whether the system is in degraded mode
func (eh *ErrorHandler) IsInDegradedMode() bool {
	eh.mu.RLock()
	defer eh.mu.RUnlock()

	if !eh.degraded {
		return false
	}

	if time.Now().After(eh.degradedUntil) {
		eh.degraded = false
		eh.logger.LogInfo("Degraded mode automatically disabled", nil)
		return false
	}

	return true
}

// DisableDegradedMode manually disables degraded mode
func (eh *ErrorHandler) DisableDegradedMode() {
	eh.mu.Lock()
	defer eh.mu.Unlock()

	if eh.degraded {
		eh.degraded = false
		eh.logger.LogInfo("Degraded mode manually disabled", nil)
	}
}

// calculateRetryDelay calculates the delay for the next retry attempt
func (eh *ErrorHandler) calculateRetryDelay(policy *RetryPolicy, retryCount int) time.Duration {
	delay := policy.InitialDelay

	// Apply exponential backoff
	for i := 0; i < retryCount; i++ {
		delay = time.Duration(float64(delay) * policy.BackoffFactor)
	}

	// Cap at maximum delay
	if delay > policy.MaxDelay {
		delay = policy.MaxDelay
	}

	// Add jitter if enabled
	if policy.Jitter && delay > 0 {
		jitterAmount := time.Duration(float64(delay) * 0.1) // 10% jitter
		if jitterAmount > 0 {
			// Simple jitter: ±10%
			jitter := time.Duration(time.Now().UnixNano()%int64(jitterAmount*2)) - jitterAmount
			delay += jitter
		}
	}

	return delay
}

// updateErrorStats updates error statistics
func (eh *ErrorHandler) updateErrorStats(errorInfo *ErrorInfo) {
	stats := eh.getOrCreateErrorStats(errorInfo.Type)
	stats.TotalCount++
	stats.LastOccurred = time.Now()

	if errorInfo.RetryCount > 0 {
		stats.RetryCount++
	}
}

// getOrCreateErrorStats gets or creates error statistics for a type
func (eh *ErrorHandler) getOrCreateErrorStats(errorType ErrorType) *ErrorStats {
	if stats, exists := eh.errorStats[errorType]; exists {
		return stats
	}

	stats := &ErrorStats{
		TotalCount:   0,
		RetryCount:   0,
		SuccessCount: 0,
		LastOccurred: time.Time{},
		LastSuccess:  time.Time{},
	}
	eh.errorStats[errorType] = stats
	return stats
}

// logError logs an error with structured information
func (eh *ErrorHandler) logError(errorInfo *ErrorInfo) {
	logData := map[string]interface{}{
		"type":        errorInfo.Type,
		"severity":    errorInfo.Severity,
		"message":     errorInfo.Message,
		"context":     errorInfo.Context,
		"timestamp":   errorInfo.Timestamp.Format(time.RFC3339),
		"retryable":   errorInfo.Retryable,
		"retry_count": errorInfo.RetryCount,
		"max_retries": errorInfo.MaxRetries,
		"component":   errorInfo.Component,
		"operation":   errorInfo.Operation,
	}

	if errorInfo.Details != nil {
		logData["details"] = errorInfo.Details
	}

	switch errorInfo.Severity {
	case SeverityCritical:
		eh.logger.LogError("CRITICAL ERROR", logData)
	case SeverityHigh:
		eh.logger.LogError("HIGH SEVERITY ERROR", logData)
	case SeverityMedium:
		eh.logger.LogWarning("MEDIUM SEVERITY ERROR", logData)
	case SeverityLow:
		eh.logger.LogInfo("LOW SEVERITY ERROR", logData)
	default:
		eh.logger.LogError("ERROR", logData)
	}
}

// LogInfo logs an informational message
func (sl *StructuredLogger) LogInfo(message string, data map[string]interface{}) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	logEntry := fmt.Sprintf("[INFO] %s", message)
	if data != nil {
		logEntry += fmt.Sprintf(" | Data: %+v", data)
	}
	sl.logger.Println(logEntry)
}

// LogWarning logs a warning message
func (sl *StructuredLogger) LogWarning(message string, data map[string]interface{}) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	logEntry := fmt.Sprintf("[WARNING] %s", message)
	if data != nil {
		logEntry += fmt.Sprintf(" | Data: %+v", data)
	}
	sl.logger.Println(logEntry)
}

// LogError logs an error message
func (sl *StructuredLogger) LogError(message string, data map[string]interface{}) {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	logEntry := fmt.Sprintf("[ERROR] %s", message)
	if data != nil {
		logEntry += fmt.Sprintf(" | Data: %+v", data)
	}
	sl.logger.Println(logEntry)
}

// CreateErrorInfo creates a new ErrorInfo instance
func CreateErrorInfo(errorType ErrorType, severity ErrorSeverity, message, context, component, operation string) *ErrorInfo {
	return &ErrorInfo{
		Type:       errorType,
		Severity:   severity,
		Message:    message,
		Context:    context,
		Timestamp:  time.Now(),
		Retryable:  true, // Default to retryable
		RetryCount: 0,
		Component:  component,
		Operation:  operation,
	}
}

// CreateNonRetryableError creates a non-retryable error
func CreateNonRetryableError(errorType ErrorType, severity ErrorSeverity, message, context, component, operation string) *ErrorInfo {
	errorInfo := CreateErrorInfo(errorType, severity, message, context, component, operation)
	errorInfo.Retryable = false
	return errorInfo
}
