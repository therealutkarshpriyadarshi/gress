package cep

import (
	"testing"
	"time"
)

// TestSequencePattern tests basic sequence pattern matching
func TestSequencePattern(t *testing.T) {
	pattern := NewPattern("test-001", "Simple Sequence").
		Sequence(
			Event("a", "event_a"),
			Event("b", "event_b"),
			Event("c", "event_c"),
		).
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()
	matchFound := false

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-001" {
			matchFound = true
		}
	})

	// Send events in sequence
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_b",
		"timestamp": baseTime.Add(1 * time.Minute),
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_c",
		"timestamp": baseTime.Add(2 * time.Minute),
	})

	if !matchFound {
		t.Error("Expected sequence pattern to match")
	}
}

// TestNegationPattern tests negation patterns (A not followed by B)
func TestNegationPattern(t *testing.T) {
	pattern := NewPattern("test-002", "Negation Test").
		Sequence(
			Event("a", "event_a"),
		).
		NotFollowedBy(
			Event("b", "event_b"),
		).
		FollowedBy(
			Event("c", "event_c"),
		).
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()
	matchCount := 0

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-002" {
			matchCount++
		}
	})

	// Test case 1: A -> C (should match, no B in between)
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_c",
		"timestamp": baseTime.Add(1 * time.Minute),
	})

	time.Sleep(100 * time.Millisecond) // Allow processing

	// Reset for test case 2
	engine = NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)
	baseTime = time.Now()

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-002" {
			t.Error("Pattern should not match when B occurs between A and C")
		}
	})

	// Test case 2: A -> B -> C (should NOT match, B violates negation)
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_b",
		"timestamp": baseTime.Add(30 * time.Second),
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_c",
		"timestamp": baseTime.Add(1 * time.Minute),
	})
}

// TestIterationPattern tests iteration patterns (event occurs N times)
func TestIterationPattern(t *testing.T) {
	pattern := NewPattern("test-003", "Iteration Test").
		Sequence(
			Event("repeated", "login_failed").Times(3),
		).
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()
	matchFound := false

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-003" {
			if len(match.MatchedEvents) == 3 {
				matchFound = true
			}
		}
	})

	// Send 3 login_failed events
	for i := 0; i < 3; i++ {
		engine.ProcessEvent(map[string]interface{}{
			"type":      "login_failed",
			"timestamp": baseTime.Add(time.Duration(i*30) * time.Second),
		})
	}

	time.Sleep(100 * time.Millisecond)

	if !matchFound {
		t.Error("Expected iteration pattern to match after 3 events")
	}
}

// TestTemporalConstraints tests temporal within constraints
func TestTemporalConstraints(t *testing.T) {
	pattern := NewPattern("test-004", "Temporal Test").
		Sequence(
			Event("a", "event_a"),
			Event("b", "event_b").Within(30*time.Second),
		).
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()

	// Test case 1: Events within temporal constraint (should match)
	matchFound := false
	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-004" {
			matchFound = true
		}
	})

	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_b",
		"timestamp": baseTime.Add(20 * time.Second), // Within 30 seconds
	})

	time.Sleep(100 * time.Millisecond)

	if !matchFound {
		t.Error("Expected pattern to match when events are within temporal constraint")
	}

	// Test case 2: Events outside temporal constraint (should NOT match)
	engine = NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)
	baseTime = time.Now()

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-004" {
			t.Error("Pattern should not match when temporal constraint is violated")
		}
	})

	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_b",
		"timestamp": baseTime.Add(40 * time.Second), // Outside 30 seconds
	})

	time.Sleep(100 * time.Millisecond)
}

// TestConditionEvaluation tests condition evaluation between events
func TestConditionEvaluation(t *testing.T) {
	pattern := NewPattern("test-005", "Condition Test").
		Sequence(
			Event("login", "login"),
			Event("transaction", "transaction"),
		).
		WithCondition("login", "transaction", "user_id", "eq").
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()

	// Test case 1: Same user_id (should match)
	matchFound := false
	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-005" {
			matchFound = true
		}
	})

	engine.ProcessEvent(map[string]interface{}{
		"type":      "login",
		"name":      "login",
		"user_id":   "user123",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "transaction",
		"name":      "transaction",
		"user_id":   "user123",
		"timestamp": baseTime.Add(1 * time.Minute),
	})

	time.Sleep(100 * time.Millisecond)

	if !matchFound {
		t.Error("Expected pattern to match when condition is satisfied")
	}

	// Test case 2: Different user_id (should NOT match)
	engine = NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)
	baseTime = time.Now()

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-005" {
			t.Error("Pattern should not match when condition is not satisfied")
		}
	})

	engine.ProcessEvent(map[string]interface{}{
		"type":      "login",
		"name":      "login",
		"user_id":   "user123",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "transaction",
		"name":      "transaction",
		"user_id":   "user456", // Different user
		"timestamp": baseTime.Add(1 * time.Minute),
	})

	time.Sleep(100 * time.Millisecond)
}

// TestTimeWindow tests pattern time window constraints
func TestTimeWindow(t *testing.T) {
	pattern := NewPattern("test-006", "Time Window Test").
		Sequence(
			Event("a", "event_a"),
			Event("b", "event_b"),
		).
		Within(1 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()

	// Test case 1: Events within window (should match)
	matchFound := false
	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-006" {
			matchFound = true
		}
	})

	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_b",
		"timestamp": baseTime.Add(45 * time.Second),
	})

	time.Sleep(100 * time.Millisecond)

	if !matchFound {
		t.Error("Expected pattern to match within time window")
	}

	// Test case 2: Events outside window (should NOT match)
	engine = NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)
	baseTime = time.Now()

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-006" {
			t.Error("Pattern should not match outside time window")
		}
	})

	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_b",
		"timestamp": baseTime.Add(90 * time.Second), // Outside 1 minute window
	})

	time.Sleep(100 * time.Millisecond)
}

// TestAtLeastQuantifier tests AtLeast quantifier
func TestAtLeastQuantifier(t *testing.T) {
	pattern := NewPattern("test-007", "AtLeast Test").
		Sequence(
			Event("clicks", "button_click").AtLeast(3),
		).
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()
	matchFound := false

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-007" {
			matchFound = true
		}
	})

	// Send 4 clicks (more than minimum 3)
	for i := 0; i < 4; i++ {
		engine.ProcessEvent(map[string]interface{}{
			"type":      "button_click",
			"timestamp": baseTime.Add(time.Duration(i*10) * time.Second),
		})
	}

	time.Sleep(100 * time.Millisecond)

	if !matchFound {
		t.Error("Expected AtLeast pattern to match with 4 events (minimum 3)")
	}
}

// TestOptionalEvent tests optional events in patterns
func TestOptionalEvent(t *testing.T) {
	pattern := NewPattern("test-008", "Optional Event Test").
		Sequence(
			Event("a", "event_a"),
			Event("b", "event_b").Optional(),
			Event("c", "event_c"),
		).
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()
	matchCount := 0

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-008" {
			matchCount++
		}
	})

	// Test case 1: A -> C (skipping optional B)
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_c",
		"timestamp": baseTime.Add(1 * time.Minute),
	})

	time.Sleep(100 * time.Millisecond)

	if matchCount != 1 {
		t.Error("Expected pattern to match when optional event is skipped")
	}

	// Test case 2: A -> B -> C (including optional B)
	engine = NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)
	baseTime = time.Now()
	matchCount = 0

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "test-008" {
			matchCount++
		}
	})

	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_a",
		"timestamp": baseTime,
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_b",
		"timestamp": baseTime.Add(30 * time.Second),
	})
	engine.ProcessEvent(map[string]interface{}{
		"type":      "event_c",
		"timestamp": baseTime.Add(1 * time.Minute),
	})

	time.Sleep(100 * time.Millisecond)

	if matchCount != 1 {
		t.Error("Expected pattern to match when optional event is included")
	}
}

// TestFraudDetectionPattern tests a realistic fraud detection pattern
func TestFraudDetectionPattern(t *testing.T) {
	pattern := NewPattern("fraud-test", "Multiple Failed Logins").
		Sequence(
			Event("failed", "login_failed").Times(3),
			Event("success", "login_success").Within(1*time.Minute),
		).
		WithCondition("failed", "success", "user_id", "eq").
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(100, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()
	matchFound := false

	engine.OnMatch(func(match MatchedPattern) {
		if match.PatternID == "fraud-test" {
			matchFound = true
		}
	})

	// Simulate credential stuffing attack
	userID := "test_user"

	// 3 failed logins
	for i := 0; i < 3; i++ {
		engine.ProcessEvent(map[string]interface{}{
			"type":      "login_failed",
			"name":      "failed",
			"user_id":   userID,
			"timestamp": baseTime.Add(time.Duration(i*10) * time.Second),
		})
	}

	// Successful login
	engine.ProcessEvent(map[string]interface{}{
		"type":      "login_success",
		"name":      "success",
		"user_id":   userID,
		"timestamp": baseTime.Add(45 * time.Second),
	})

	time.Sleep(100 * time.Millisecond)

	if !matchFound {
		t.Error("Expected fraud detection pattern to match")
	}
}

// TestEngineStatistics tests engine statistics reporting
func TestEngineStatistics(t *testing.T) {
	engine := NewCEPEngine(100, 10*time.Minute)

	pattern1 := NewPattern("stat-test-1", "Test Pattern 1").
		Sequence(Event("a", "event_a")).
		Build()

	pattern2 := NewPattern("stat-test-2", "Test Pattern 2").
		Sequence(Event("b", "event_b")).
		Build()

	engine.RegisterPattern(pattern1)
	engine.RegisterPattern(pattern2)

	stats := engine.GetStatistics()

	if stats["registered_patterns"] != 2 {
		t.Errorf("Expected 2 registered patterns, got %v", stats["registered_patterns"])
	}
}

// TestPatternUnregistration tests pattern removal
func TestPatternUnregistration(t *testing.T) {
	engine := NewCEPEngine(100, 10*time.Minute)

	pattern := NewPattern("unreg-test", "Unregister Test").
		Sequence(Event("a", "event_a")).
		Build()

	engine.RegisterPattern(pattern)
	stats := engine.GetStatistics()

	if stats["registered_patterns"] != 1 {
		t.Error("Expected 1 registered pattern")
	}

	engine.UnregisterPattern("unreg-test")
	stats = engine.GetStatistics()

	if stats["registered_patterns"] != 0 {
		t.Error("Expected 0 registered patterns after unregistration")
	}
}

// BenchmarkSequenceMatching benchmarks sequence pattern matching
func BenchmarkSequenceMatching(b *testing.B) {
	pattern := NewPattern("bench-001", "Benchmark Sequence").
		Sequence(
			Event("a", "event_a"),
			Event("b", "event_b"),
			Event("c", "event_c"),
		).
		Within(5 * time.Minute).
		Build()

	engine := NewCEPEngine(1000, 10*time.Minute)
	engine.RegisterPattern(pattern)

	baseTime := time.Now()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.ProcessEvent(map[string]interface{}{
			"type":      "event_a",
			"timestamp": baseTime,
		})
		engine.ProcessEvent(map[string]interface{}{
			"type":      "event_b",
			"timestamp": baseTime.Add(1 * time.Second),
		})
		engine.ProcessEvent(map[string]interface{}{
			"type":      "event_c",
			"timestamp": baseTime.Add(2 * time.Second),
		})
	}
}
