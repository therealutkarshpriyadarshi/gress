package scaler

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	gressv1alpha1 "github.com/therealutkarshpriyadarshi/gress/operator/api/v1alpha1"
)

// KafkaLagScaler handles auto-scaling based on Kafka consumer lag
type KafkaLagScaler struct {
	// Configuration
	CheckInterval time.Duration
	LagThreshold  int64
}

// NewKafkaLagScaler creates a new Kafka lag-based scaler
func NewKafkaLagScaler() *KafkaLagScaler {
	return &KafkaLagScaler{
		CheckInterval: 30 * time.Second,
		LagThreshold:  10000, // Default threshold
	}
}

// CalculateDesiredReplicas calculates the desired number of replicas based on Kafka lag
func (k *KafkaLagScaler) CalculateDesiredReplicas(ctx context.Context, streamJob *gressv1alpha1.StreamJob) (int32, error) {
	logger := log.FromContext(ctx)

	if streamJob.Spec.AutoScaler == nil || !streamJob.Spec.AutoScaler.Enabled {
		return streamJob.Spec.Parallelism, nil
	}

	// Find Kafka source configurations
	kafkaSources := k.extractKafkaSources(streamJob)
	if len(kafkaSources) == 0 {
		logger.Info("No Kafka sources found, skipping lag-based scaling")
		return streamJob.Spec.Parallelism, nil
	}

	// Calculate total lag across all Kafka sources
	totalLag, err := k.calculateTotalLag(ctx, kafkaSources)
	if err != nil {
		return streamJob.Spec.Parallelism, err
	}

	// Calculate desired replicas based on lag
	desiredReplicas := k.scaleBasedOnLag(totalLag, streamJob)

	logger.Info("Calculated desired replicas based on Kafka lag",
		"totalLag", totalLag,
		"currentReplicas", streamJob.Status.CurrentReplicas,
		"desiredReplicas", desiredReplicas)

	return desiredReplicas, nil
}

// extractKafkaSources extracts Kafka source configurations from StreamJob
func (k *KafkaLagScaler) extractKafkaSources(streamJob *gressv1alpha1.StreamJob) []*gressv1alpha1.KafkaSourceSpec {
	kafkaSources := []*gressv1alpha1.KafkaSourceSpec{}

	for _, source := range streamJob.Spec.Sources {
		if source.Type == "kafka" && source.Kafka != nil {
			kafkaSources = append(kafkaSources, source.Kafka)
		}
	}

	return kafkaSources
}

// calculateTotalLag calculates total consumer lag across all Kafka sources
func (k *KafkaLagScaler) calculateTotalLag(ctx context.Context, sources []*gressv1alpha1.KafkaSourceSpec) (int64, error) {
	logger := log.FromContext(ctx)

	// This would query Kafka for consumer lag using:
	// 1. Kafka Admin API
	// 2. Prometheus metrics (kafka_consumergroup_lag)
	// 3. Burrow API
	// 4. JMX metrics

	// For now, return a simulated value
	// In production, implement actual Kafka lag querying

	logger.Info("Querying Kafka lag", "sources", len(sources))

	// Simulated lag calculation
	var totalLag int64 = 5000 // Placeholder

	return totalLag, nil
}

// scaleBasedOnLag determines desired replicas based on lag
func (k *KafkaLagScaler) scaleBasedOnLag(lag int64, streamJob *gressv1alpha1.StreamJob) int32 {
	minReplicas := streamJob.Spec.AutoScaler.MinReplicas
	maxReplicas := streamJob.Spec.AutoScaler.MaxReplicas
	currentReplicas := streamJob.Status.CurrentReplicas

	if currentReplicas == 0 {
		currentReplicas = streamJob.Spec.Parallelism
	}

	// Simple scaling algorithm:
	// - If lag > threshold * 2: scale up by 2x
	// - If lag > threshold: scale up by 50%
	// - If lag < threshold / 2: scale down by 25%

	var desiredReplicas int32

	if lag > k.LagThreshold*2 {
		desiredReplicas = currentReplicas * 2
	} else if lag > k.LagThreshold {
		desiredReplicas = currentReplicas + currentReplicas/2
	} else if lag < k.LagThreshold/2 {
		desiredReplicas = currentReplicas - currentReplicas/4
	} else {
		desiredReplicas = currentReplicas
	}

	// Apply min/max constraints
	if desiredReplicas < minReplicas {
		desiredReplicas = minReplicas
	}
	if desiredReplicas > maxReplicas {
		desiredReplicas = maxReplicas
	}

	return desiredReplicas
}

// GetScalingMetrics returns current scaling metrics
func (k *KafkaLagScaler) GetScalingMetrics(ctx context.Context, streamJob *gressv1alpha1.StreamJob) (map[string]interface{}, error) {
	kafkaSources := k.extractKafkaSources(streamJob)
	if len(kafkaSources) == 0 {
		return nil, fmt.Errorf("no Kafka sources found")
	}

	totalLag, err := k.calculateTotalLag(ctx, kafkaSources)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"kafka_lag":         totalLag,
		"lag_threshold":     k.LagThreshold,
		"current_replicas":  streamJob.Status.CurrentReplicas,
		"desired_replicas":  k.scaleBasedOnLag(totalLag, streamJob),
	}, nil
}
