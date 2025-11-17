package errors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
)

// ErrorCategory represents the type of error for handling purposes
type ErrorCategory int

const (
	// CategoryRetriable indicates the operation can be retried with exponential backoff
	CategoryRetriable ErrorCategory = iota
	// CategoryFatal indicates the operation should not be retried and requires intervention
	CategoryFatal
	// CategoryTransient indicates the operation can be retried immediately or after brief delay
	CategoryTransient
)

func (c ErrorCategory) String() string {
	switch c {
	case CategoryRetriable:
		return "retriable"
	case CategoryFatal:
		return "fatal"
	case CategoryTransient:
		return "transient"
	default:
		return "unknown"
	}
}

// ClassifiedError wraps an error with its category and additional context
type ClassifiedError struct {
	Err      error
	Category ErrorCategory
	Message  string
	Metadata map[string]interface{}
}

func (ce *ClassifiedError) Error() string {
	if ce.Message != "" {
		return fmt.Sprintf("%s: %v", ce.Message, ce.Err)
	}
	return ce.Err.Error()
}

func (ce *ClassifiedError) Unwrap() error {
	return ce.Err
}

// NewClassifiedError creates a new classified error with category
func NewClassifiedError(err error, category ErrorCategory, message string) *ClassifiedError {
	return &ClassifiedError{
		Err:      err,
		Category: category,
		Message:  message,
		Metadata: make(map[string]interface{}),
	}
}

// WithMetadata adds metadata to the classified error
func (ce *ClassifiedError) WithMetadata(key string, value interface{}) *ClassifiedError {
	ce.Metadata[key] = value
	return ce
}

// ClassifyError categorizes an error based on its type and characteristics
func ClassifyError(err error) ErrorCategory {
	if err == nil {
		return CategoryFatal // Should not happen, but treat as fatal
	}

	// Check if already classified
	var classified *ClassifiedError
	if errors.As(err, &classified) {
		return classified.Category
	}

	// Context errors - typically not retriable
	if errors.Is(err, context.Canceled) {
		return CategoryFatal
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return CategoryRetriable
	}

	// IO errors - often transient
	if errors.Is(err, io.EOF) {
		return CategoryRetriable
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return CategoryRetriable
	}

	// Network errors - usually retriable
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return CategoryRetriable
		}
		if netErr.Temporary() {
			return CategoryTransient
		}
		return CategoryRetriable
	}

	// System call errors
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return classifySyscallError(errno)
	}

	// Check for common error messages
	errMsg := err.Error()
	if containsAny(errMsg, []string{
		"connection refused",
		"connection reset",
		"broken pipe",
		"no route to host",
		"network is unreachable",
	}) {
		return CategoryRetriable
	}

	if containsAny(errMsg, []string{
		"invalid argument",
		"invalid syntax",
		"parse error",
		"unmarshal",
		"marshal",
	}) {
		return CategoryFatal
	}

	if containsAny(errMsg, []string{
		"timeout",
		"deadline exceeded",
		"too many open files",
		"resource temporarily unavailable",
	}) {
		return CategoryRetriable
	}

	// Default to retriable for unknown errors
	return CategoryRetriable
}

// classifySyscallError categorizes system call errors
func classifySyscallError(errno syscall.Errno) ErrorCategory {
	switch errno {
	// Network-related retriable errors
	case syscall.ECONNREFUSED,
		syscall.ECONNRESET,
		syscall.ECONNABORTED,
		syscall.ENETUNREACH,
		syscall.EHOSTUNREACH,
		syscall.ETIMEDOUT,
		syscall.EPIPE:
		return CategoryRetriable

	// Transient resource errors
	case syscall.EAGAIN,
		syscall.EWOULDBLOCK,
		syscall.EMFILE,
		syscall.ENFILE:
		return CategoryTransient

	// Fatal errors
	case syscall.EINVAL,
		syscall.EACCES,
		syscall.EPERM,
		syscall.ENOENT,
		syscall.EEXIST:
		return CategoryFatal

	default:
		return CategoryRetriable
	}
}

// IsRetriable checks if an error can be retried
func IsRetriable(err error) bool {
	category := ClassifyError(err)
	return category == CategoryRetriable || category == CategoryTransient
}

// IsFatal checks if an error is fatal and should not be retried
func IsFatal(err error) bool {
	return ClassifyError(err) == CategoryFatal
}

// IsTransient checks if an error is transient
func IsTransient(err error) bool {
	return ClassifyError(err) == CategoryTransient
}

// containsAny checks if a string contains any of the substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	// Simple case-insensitive check
	sLower := toLower(s)
	substrLower := toLower(substr)
	return containsExact(sLower, substrLower)
}

// containsExact checks if a string contains a substring (case-sensitive)
func containsExact(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

// findSubstring finds the index of a substring
func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// toLower converts a string to lowercase
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}
