package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/therealutkarshpriyadarshi/gress/pkg/cep"
)

// FraudDetectionExample demonstrates CEP for fraud detection
func main() {
	fmt.Println("=== Fraud Detection CEP Example ===\n")

	// Create CEP engine with 5-minute timeout
	engine := cep.NewCEPEngine(1000, 5*time.Minute)

	// Register multiple fraud detection patterns
	registerFraudPatterns(engine)

	// Set up match callback
	engine.OnMatch(func(match cep.MatchedPattern) {
		fmt.Printf("\n🚨 ALERT: %s detected!\n", match.PatternName)
		fmt.Printf("   Pattern ID: %s\n", match.PatternID)
		fmt.Printf("   Severity: %s\n", match.Severity)
		fmt.Printf("   Time Range: %s to %s\n",
			match.FirstEventTime.Format("15:04:05"),
			match.LastEventTime.Format("15:04:05"))
		fmt.Printf("   Events Matched: %d\n", len(match.MatchedEvents))

		// Print metadata
		if metadata, ok := match.Metadata["unique_users"].(int); ok && metadata > 0 {
			fmt.Printf("   Unique Users: %d\n", metadata)
		}
		if metadata, ok := match.Metadata["unique_ips"].(int); ok && metadata > 0 {
			fmt.Printf("   Unique IPs: %d\n", metadata)
		}

		// Print event details
		fmt.Println("\n   Event Details:")
		for i, event := range match.MatchedEvents {
			eventJSON, _ := json.MarshalIndent(event, "   ", "  ")
			fmt.Printf("   Event %d: %s\n", i+1, eventJSON)
		}
		fmt.Println()
	})

	// Simulate fraud scenarios
	fmt.Println("Simulating fraud detection scenarios...\n")

	// Scenario 1: Multiple failed logins followed by success (Credential Stuffing)
	fmt.Println("Scenario 1: Credential Stuffing Attack")
	simulateCredentialStuffing(engine)

	time.Sleep(2 * time.Second)

	// Scenario 2: High-value transaction from new location without 2FA
	fmt.Println("\nScenario 2: Suspicious Transaction from New Location")
	simulateSuspiciousTransaction(engine)

	time.Sleep(2 * time.Second)

	// Scenario 3: Rapid account creation from same IP
	fmt.Println("\nScenario 3: Automated Account Creation")
	simulateAccountCreationFraud(engine)

	time.Sleep(2 * time.Second)

	// Scenario 4: Velocity fraud - many transactions in short time
	fmt.Println("\nScenario 4: Transaction Velocity Fraud")
	simulateVelocityFraud(engine)

	time.Sleep(2 * time.Second)

	// Print engine statistics
	stats := engine.GetStatistics()
	fmt.Println("\n=== CEP Engine Statistics ===")
	for key, value := range stats {
		fmt.Printf("%s: %v\n", key, value)
	}
}

// registerFraudPatterns registers various fraud detection patterns
func registerFraudPatterns(engine *cep.CEPEngine) {
	// Pattern 1: Multiple failed login attempts followed by success
	credentialStuffing := cep.NewPattern("fraud-001", "Credential Stuffing Attack").
		WithDescription("3+ failed logins followed by successful login within 5 minutes").
		Sequence(
			cep.Event("failed_login", "login_failed").Times(3),
			cep.Event("successful_login", "login_success").Within(1*time.Minute),
		).
		WithCondition("failed_login", "successful_login", "user_id", "eq").
		Within(5 * time.Minute).
		Build()

	engine.RegisterPattern(credentialStuffing)

	// Pattern 2: Login from new location + high-value transaction without 2FA
	suspiciousTransaction := cep.NewPattern("fraud-002", "Suspicious High-Value Transaction").
		WithDescription("Login from new location + high-value transaction without 2FA").
		Sequence(
			cep.Event("new_location_login", "login_success").
				With("new_location", true),
			cep.Event("high_value_tx", "transaction_initiated").
				With("high_value", true).
				Within(10*time.Minute),
		).
		NotFollowedBy(
			cep.Event("2fa", "two_factor_auth").
				With("verified", true),
		).
		WithCondition("new_location_login", "high_value_tx", "user_id", "eq").
		Within(30 * time.Minute).
		Build()

	engine.RegisterPattern(suspiciousTransaction)

	// Pattern 3: Rapid account creation from same IP
	accountCreationFraud := cep.NewPattern("fraud-003", "Automated Account Creation").
		WithDescription("5+ accounts created from same IP within 10 minutes").
		Sequence(
			cep.Event("account_creation", "account_created").AtLeast(5),
		).
		Within(10 * time.Minute).
		Build()

	engine.RegisterPattern(accountCreationFraud)

	// Pattern 4: Transaction velocity fraud
	velocityFraud := cep.NewPattern("fraud-004", "Transaction Velocity Fraud").
		WithDescription("Multiple high-value transactions in rapid succession").
		Sequence(
			cep.Event("transaction", "transaction_completed").
				With("high_value", true).
				AtLeast(4),
		).
		Within(3 * time.Minute).
		Build()

	engine.RegisterPattern(velocityFraud)

	// Pattern 5: Account takeover pattern
	accountTakeover := cep.NewPattern("fraud-005", "Account Takeover").
		WithDescription("Password change followed by email change and withdrawal").
		Sequence(
			cep.Event("pwd_change", "password_changed"),
			cep.Event("email_change", "email_changed").Within(5*time.Minute),
			cep.Event("withdrawal", "withdrawal_initiated").Within(10*time.Minute),
		).
		Within(15 * time.Minute).
		Build()

	engine.RegisterPattern(accountTakeover)
}

// simulateCredentialStuffing simulates a credential stuffing attack
func simulateCredentialStuffing(engine *cep.CEPEngine) {
	userID := "user123"
	ip := "192.168.1.100"
	baseTime := time.Now()

	// 3 failed login attempts
	for i := 0; i < 3; i++ {
		event := map[string]interface{}{
			"type":      "login_failed",
			"user_id":   userID,
			"ip_address": ip,
			"timestamp": baseTime.Add(time.Duration(i*10) * time.Second),
			"reason":    "invalid_password",
		}
		engine.ProcessEvent(event)
		fmt.Printf("  ⚠️  Failed login attempt %d for user %s\n", i+1, userID)
	}

	// Successful login after 30 seconds
	successEvent := map[string]interface{}{
		"type":      "login_success",
		"user_id":   userID,
		"ip_address": ip,
		"timestamp": baseTime.Add(30 * time.Second),
	}
	engine.ProcessEvent(successEvent)
	fmt.Printf("  ✅ Successful login for user %s\n", userID)
}

// simulateSuspiciousTransaction simulates suspicious transaction from new location
func simulateSuspiciousTransaction(engine *cep.CEPEngine) {
	userID := "user456"
	newIP := "203.0.113.50" // New location IP
	baseTime := time.Now()

	// Login from new location
	loginEvent := map[string]interface{}{
		"type":         "login_success",
		"user_id":      userID,
		"ip_address":   newIP,
		"new_location": true,
		"location":     "Unknown Country",
		"timestamp":    baseTime,
	}
	engine.ProcessEvent(loginEvent)
	fmt.Printf("  🌍 Login from new location for user %s (IP: %s)\n", userID, newIP)

	// High-value transaction without 2FA
	txEvent := map[string]interface{}{
		"type":       "transaction_initiated",
		"user_id":    userID,
		"amount":     15000.00,
		"high_value": true,
		"timestamp":  baseTime.Add(2 * time.Minute),
	}
	engine.ProcessEvent(txEvent)
	fmt.Printf("  💰 High-value transaction initiated: $%.2f\n", txEvent["amount"])
}

// simulateAccountCreationFraud simulates automated account creation
func simulateAccountCreationFraud(engine *cep.CEPEngine) {
	ip := "198.51.100.25"
	baseTime := time.Now()

	// Create 6 accounts rapidly from same IP
	for i := 0; i < 6; i++ {
		event := map[string]interface{}{
			"type":       "account_created",
			"user_id":    fmt.Sprintf("bot_user_%d", i),
			"ip_address": ip,
			"timestamp":  baseTime.Add(time.Duration(i*30) * time.Second),
			"email":      fmt.Sprintf("bot%d@example.com", i),
		}
		engine.ProcessEvent(event)
		fmt.Printf("  👤 Account created: bot_user_%d from IP %s\n", i, ip)
	}
}

// simulateVelocityFraud simulates rapid transaction velocity fraud
func simulateVelocityFraud(engine *cep.CEPEngine) {
	userID := "user789"
	baseTime := time.Now()

	// Process 5 high-value transactions rapidly
	for i := 0; i < 5; i++ {
		event := map[string]interface{}{
			"type":       "transaction_completed",
			"user_id":    userID,
			"amount":     5000.00 + float64(i*1000),
			"high_value": true,
			"timestamp":  baseTime.Add(time.Duration(i*20) * time.Second),
			"merchant":   fmt.Sprintf("Merchant_%d", i),
		}
		engine.ProcessEvent(event)
		fmt.Printf("  💳 Transaction %d: $%.2f at Merchant_%d\n", i+1, event["amount"], i)
	}
}
