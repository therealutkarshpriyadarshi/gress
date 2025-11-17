package errors

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected ErrorCategory
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: CategoryFatal,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: CategoryFatal,
		},
		{
			name:     "context deadline exceeded",
			err:      context.DeadlineExceeded,
			expected: CategoryRetriable,
		},
		{
			name:     "EOF",
			err:      io.EOF,
			expected: CategoryRetriable,
		},
		{
			name:     "unexpected EOF",
			err:      io.ErrUnexpectedEOF,
			expected: CategoryRetriable,
		},
		{
			name:     "connection refused",
			err:      syscall.ECONNREFUSED,
			expected: CategoryRetriable,
		},
		{
			name:     "connection reset",
			err:      syscall.ECONNRESET,
			expected: CategoryRetriable,
		},
		{
			name:     "EAGAIN",
			err:      syscall.EAGAIN,
			expected: CategoryTransient,
		},
		{
			name:     "invalid argument",
			err:      syscall.EINVAL,
			expected: CategoryFatal,
		},
		{
			name:     "permission denied",
			err:      syscall.EACCES,
			expected: CategoryFatal,
		},
		{
			name:     "generic error with connection refused",
			err:      errors.New("connection refused"),
			expected: CategoryRetriable,
		},
		{
			name:     "generic error with timeout",
			err:      errors.New("operation timeout"),
			expected: CategoryRetriable,
		},
		{
			name:     "generic error with parse error",
			err:      errors.New("parse error: invalid syntax"),
			expected: CategoryFatal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ClassifyError(tt.err)
			if result != tt.expected {
				t.Errorf("ClassifyError(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestClassifiedError(t *testing.T) {
	originalErr := errors.New("test error")
	classified := NewClassifiedError(originalErr, CategoryRetriable, "operation failed")

	// Test error message
	expectedMsg := "operation failed: test error"
	if classified.Error() != expectedMsg {
		t.Errorf("Error() = %v, want %v", classified.Error(), expectedMsg)
	}

	// Test unwrap
	if !errors.Is(classified, originalErr) {
		t.Error("Unwrap failed, classified error should wrap original error")
	}

	// Test metadata
	classified.WithMetadata("key", "value")
	if classified.Metadata["key"] != "value" {
		t.Error("Metadata not set correctly")
	}

	// Test classification
	category := ClassifyError(classified)
	if category != CategoryRetriable {
		t.Errorf("ClassifyError(classified) = %v, want %v", category, CategoryRetriable)
	}
}

func TestIsRetriable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "retriable error",
			err:      syscall.ECONNREFUSED,
			expected: true,
		},
		{
			name:     "transient error",
			err:      syscall.EAGAIN,
			expected: true,
		},
		{
			name:     "fatal error",
			err:      syscall.EINVAL,
			expected: false,
		},
		{
			name:     "context canceled",
			err:      context.Canceled,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetriable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetriable(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsFatal(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "fatal error",
			err:      syscall.EINVAL,
			expected: true,
		},
		{
			name:     "retriable error",
			err:      syscall.ECONNREFUSED,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFatal(tt.err)
			if result != tt.expected {
				t.Errorf("IsFatal(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "transient error",
			err:      syscall.EAGAIN,
			expected: true,
		},
		{
			name:     "retriable error",
			err:      syscall.ECONNREFUSED,
			expected: false,
		},
		{
			name:     "fatal error",
			err:      syscall.EINVAL,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsTransient(tt.err)
			if result != tt.expected {
				t.Errorf("IsTransient(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestNetError(t *testing.T) {
	// Create a mock network error
	mockErr := &mockNetError{timeout: true, temporary: false}

	category := ClassifyError(mockErr)
	if category != CategoryRetriable {
		t.Errorf("ClassifyError(timeout network error) = %v, want %v", category, CategoryRetriable)
	}

	mockErr = &mockNetError{timeout: false, temporary: true}
	category = ClassifyError(mockErr)
	if category != CategoryTransient {
		t.Errorf("ClassifyError(temporary network error) = %v, want %v", category, CategoryTransient)
	}
}

// mockNetError implements net.Error for testing
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock network error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

// Ensure mockNetError implements net.Error
var _ net.Error = (*mockNetError)(nil)

func TestErrorCategoryString(t *testing.T) {
	tests := []struct {
		category ErrorCategory
		expected string
	}{
		{CategoryRetriable, "retriable"},
		{CategoryFatal, "fatal"},
		{CategoryTransient, "transient"},
		{ErrorCategory(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.category.String()
			if result != tt.expected {
				t.Errorf("String() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func BenchmarkClassifyError(b *testing.B) {
	testErrors := []error{
		context.Canceled,
		context.DeadlineExceeded,
		io.EOF,
		syscall.ECONNREFUSED,
		syscall.EAGAIN,
		syscall.EINVAL,
		errors.New("connection refused"),
		errors.New("timeout"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range testErrors {
			ClassifyError(err)
		}
	}
}
