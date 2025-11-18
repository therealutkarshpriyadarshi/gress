package ml

import (
	"context"
	"testing"
	"time"
)

// MockModel is a mock implementation of the Model interface for testing
type MockModel struct {
	metadata       ModelMetadata
	predictFunc    func(ctx context.Context, features map[string]interface{}) (*PredictionResponse, error)
	predictBatchFunc func(ctx context.Context, requests []*PredictionRequest) ([]*PredictionResponse, error)
	warmupFunc     func(ctx context.Context) error
	closeFunc      func() error
}

func NewMockModel(name, version string) *MockModel {
	return &MockModel{
		metadata: ModelMetadata{
			Name:        name,
			Version:     version,
			Type:        ModelTypeCustom,
			InputNames:  []string{"input"},
			OutputNames: []string{"output"},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Tags:        make(map[string]string),
		},
	}
}

func (m *MockModel) Predict(ctx context.Context, features map[string]interface{}) (*PredictionResponse, error) {
	if m.predictFunc != nil {
		return m.predictFunc(ctx, features)
	}
	return &PredictionResponse{
		Prediction: "mock_prediction",
		Latency:    time.Millisecond,
		ModelInfo:  m.metadata,
	}, nil
}

func (m *MockModel) PredictBatch(ctx context.Context, requests []*PredictionRequest) ([]*PredictionResponse, error) {
	if m.predictBatchFunc != nil {
		return m.predictBatchFunc(ctx, requests)
	}
	responses := make([]*PredictionResponse, len(requests))
	for i := range requests {
		responses[i] = &PredictionResponse{
			Prediction: "mock_prediction",
			Latency:    time.Millisecond,
			ModelInfo:  m.metadata,
		}
	}
	return responses, nil
}

func (m *MockModel) GetMetadata() ModelMetadata {
	return m.metadata
}

func (m *MockModel) Warmup(ctx context.Context) error {
	if m.warmupFunc != nil {
		return m.warmupFunc(ctx)
	}
	return nil
}

func (m *MockModel) Close() error {
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

func TestModelRegistry_Register(t *testing.T) {
	registry := NewModelRegistry()
	model := NewMockModel("test-model", "v1")

	err := registry.Register(model)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	// Verify model is registered
	retrievedModel, err := registry.Get("test-model", "v1")
	if err != nil {
		t.Fatalf("Failed to get registered model: %v", err)
	}

	if retrievedModel.GetMetadata().Name != "test-model" {
		t.Errorf("Expected model name 'test-model', got '%s'", retrievedModel.GetMetadata().Name)
	}
}

func TestModelRegistry_RegisterDuplicate(t *testing.T) {
	registry := NewModelRegistry()
	model1 := NewMockModel("test-model", "v1")
	model2 := NewMockModel("test-model", "v1")

	err := registry.Register(model1)
	if err != nil {
		t.Fatalf("Failed to register first model: %v", err)
	}

	// Registering duplicate should fail
	err = registry.Register(model2)
	if err == nil {
		t.Error("Expected error when registering duplicate model, got nil")
	}
}

func TestModelRegistry_GetLatest(t *testing.T) {
	registry := NewModelRegistry()

	model1 := NewMockModel("test-model", "v1")
	model2 := NewMockModel("test-model", "v2")
	model3 := NewMockModel("test-model", "v3")

	registry.Register(model1)
	registry.Register(model2)
	registry.Register(model3)

	latestModel, err := registry.GetLatest("test-model")
	if err != nil {
		t.Fatalf("Failed to get latest model: %v", err)
	}

	// v3 should be latest (alphabetically)
	if latestModel.GetMetadata().Version != "v3" {
		t.Errorf("Expected latest version 'v3', got '%s'", latestModel.GetMetadata().Version)
	}
}

func TestModelRegistry_List(t *testing.T) {
	registry := NewModelRegistry()

	model1 := NewMockModel("model-a", "v1")
	model2 := NewMockModel("model-a", "v2")
	model3 := NewMockModel("model-b", "v1")

	registry.Register(model1)
	registry.Register(model2)
	registry.Register(model3)

	list := registry.List()

	if len(list) != 2 {
		t.Errorf("Expected 2 models, got %d", len(list))
	}

	if len(list["model-a"]) != 2 {
		t.Errorf("Expected 2 versions for model-a, got %d", len(list["model-a"]))
	}

	if len(list["model-b"]) != 1 {
		t.Errorf("Expected 1 version for model-b, got %d", len(list["model-b"]))
	}
}

func TestModelRegistry_Unregister(t *testing.T) {
	registry := NewModelRegistry()
	model := NewMockModel("test-model", "v1")

	err := registry.Register(model)
	if err != nil {
		t.Fatalf("Failed to register model: %v", err)
	}

	err = registry.Unregister("test-model", "v1")
	if err != nil {
		t.Fatalf("Failed to unregister model: %v", err)
	}

	// Model should no longer be available
	_, err = registry.Get("test-model", "v1")
	if err == nil {
		t.Error("Expected error when getting unregistered model, got nil")
	}
}

func TestModelRegistry_Close(t *testing.T) {
	registry := NewModelRegistry()

	closeCalled := false
	model := NewMockModel("test-model", "v1")
	model.closeFunc = func() error {
		closeCalled = true
		return nil
	}

	registry.Register(model)

	err := registry.Close()
	if err != nil {
		t.Fatalf("Failed to close registry: %v", err)
	}

	if !closeCalled {
		t.Error("Expected model's Close() to be called")
	}

	// Registry should be empty after close
	list := registry.List()
	if len(list) != 0 {
		t.Errorf("Expected empty registry after close, got %d models", len(list))
	}
}

func TestPredictionRequest(t *testing.T) {
	req := &PredictionRequest{
		Features: map[string]interface{}{
			"feature1": 1.0,
			"feature2": "test",
		},
		Timestamp: time.Now(),
		RequestID: "test-request",
	}

	if req.Features["feature1"] != 1.0 {
		t.Errorf("Expected feature1 to be 1.0, got %v", req.Features["feature1"])
	}

	if req.RequestID != "test-request" {
		t.Errorf("Expected request ID 'test-request', got '%s'", req.RequestID)
	}
}

func TestModelMetadata(t *testing.T) {
	meta := ModelMetadata{
		Name:        "test-model",
		Version:     "v1.0",
		Type:        ModelTypeONNX,
		InputNames:  []string{"input1", "input2"},
		OutputNames: []string{"output"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Tags: map[string]string{
			"environment": "production",
			"task":        "classification",
		},
		Description: "Test model",
	}

	if meta.Name != "test-model" {
		t.Errorf("Expected name 'test-model', got '%s'", meta.Name)
	}

	if meta.Type != ModelTypeONNX {
		t.Errorf("Expected type ONNX, got '%s'", meta.Type)
	}

	if len(meta.InputNames) != 2 {
		t.Errorf("Expected 2 inputs, got %d", len(meta.InputNames))
	}

	if meta.Tags["environment"] != "production" {
		t.Errorf("Expected environment tag 'production', got '%s'", meta.Tags["environment"])
	}
}
