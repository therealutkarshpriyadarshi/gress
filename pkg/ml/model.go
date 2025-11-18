package ml

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ModelType represents the type of ML model
type ModelType string

const (
	ModelTypeONNX       ModelType = "onnx"
	ModelTypeTensorFlow ModelType = "tensorflow"
	ModelTypeScikit     ModelType = "scikit"
	ModelTypeCustom     ModelType = "custom"
)

// ModelMetadata contains information about a model
type ModelMetadata struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Type        ModelType         `json:"type"`
	InputNames  []string          `json:"input_names"`
	OutputNames []string          `json:"output_names"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Tags        map[string]string `json:"tags"`
	Description string            `json:"description"`
}

// PredictionRequest represents a single prediction request
type PredictionRequest struct {
	Features  map[string]interface{} `json:"features"`
	Timestamp time.Time              `json:"timestamp"`
	RequestID string                 `json:"request_id"`
}

// PredictionResponse represents a prediction result
type PredictionResponse struct {
	Prediction interface{}            `json:"prediction"`
	Scores     map[string]float64     `json:"scores,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	Latency    time.Duration          `json:"latency"`
	ModelInfo  ModelMetadata          `json:"model_info"`
}

// Model is the core interface for ML models
type Model interface {
	// Predict performs inference on a single feature set
	Predict(ctx context.Context, features map[string]interface{}) (*PredictionResponse, error)

	// PredictBatch performs batch inference on multiple feature sets
	PredictBatch(ctx context.Context, requests []*PredictionRequest) ([]*PredictionResponse, error)

	// GetMetadata returns model metadata
	GetMetadata() ModelMetadata

	// Warmup performs model warmup to initialize resources
	Warmup(ctx context.Context) error

	// Close releases model resources
	Close() error
}

// OnlineModel extends Model with online learning capabilities
type OnlineModel interface {
	Model

	// PartialFit updates the model with new training data
	PartialFit(ctx context.Context, features map[string]interface{}, target interface{}) error

	// PartialFitBatch updates the model with a batch of training data
	PartialFitBatch(ctx context.Context, samples []TrainingSample) error

	// SaveCheckpoint saves the current model state
	SaveCheckpoint(ctx context.Context, path string) error

	// LoadCheckpoint loads model state from a checkpoint
	LoadCheckpoint(ctx context.Context, path string) error
}

// TrainingSample represents a training sample for online learning
type TrainingSample struct {
	Features  map[string]interface{} `json:"features"`
	Target    interface{}            `json:"target"`
	Weight    float64                `json:"weight,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// ModelRegistry manages multiple model versions
type ModelRegistry struct {
	mu     sync.RWMutex
	models map[string]map[string]Model // name -> version -> model
}

// NewModelRegistry creates a new model registry
func NewModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		models: make(map[string]map[string]Model),
	}
}

// Register registers a model with the registry
func (r *ModelRegistry) Register(model Model) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	meta := model.GetMetadata()
	if meta.Name == "" {
		return fmt.Errorf("model name cannot be empty")
	}
	if meta.Version == "" {
		return fmt.Errorf("model version cannot be empty")
	}

	if _, exists := r.models[meta.Name]; !exists {
		r.models[meta.Name] = make(map[string]Model)
	}

	if _, exists := r.models[meta.Name][meta.Version]; exists {
		return fmt.Errorf("model %s version %s already registered", meta.Name, meta.Version)
	}

	r.models[meta.Name][meta.Version] = model
	return nil
}

// Get retrieves a specific model version
func (r *ModelRegistry) Get(name, version string) (Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, exists := r.models[name]
	if !exists {
		return nil, fmt.Errorf("model %s not found", name)
	}

	model, exists := versions[version]
	if !exists {
		return nil, fmt.Errorf("model %s version %s not found", name, version)
	}

	return model, nil
}

// GetLatest retrieves the latest version of a model (highest version string)
func (r *ModelRegistry) GetLatest(name string) (Model, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	versions, exists := r.models[name]
	if !exists {
		return nil, fmt.Errorf("model %s not found", name)
	}

	var latestVersion string
	var latestModel Model
	for version, model := range versions {
		if latestVersion == "" || version > latestVersion {
			latestVersion = version
			latestModel = model
		}
	}

	return latestModel, nil
}

// List returns all registered models
func (r *ModelRegistry) List() map[string][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]string)
	for name, versions := range r.models {
		versionList := make([]string, 0, len(versions))
		for version := range versions {
			versionList = append(versionList, version)
		}
		result[name] = versionList
	}
	return result
}

// Unregister removes a model from the registry
func (r *ModelRegistry) Unregister(name, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	versions, exists := r.models[name]
	if !exists {
		return fmt.Errorf("model %s not found", name)
	}

	model, exists := versions[version]
	if !exists {
		return fmt.Errorf("model %s version %s not found", name, version)
	}

	// Close the model
	if err := model.Close(); err != nil {
		return fmt.Errorf("failed to close model: %w", err)
	}

	delete(versions, version)
	if len(versions) == 0 {
		delete(r.models, name)
	}

	return nil
}

// Close closes all registered models
func (r *ModelRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var errs []error
	for name, versions := range r.models {
		for version, model := range versions {
			if err := model.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close model %s:%s: %w", name, version, err))
			}
		}
	}

	r.models = make(map[string]map[string]Model)

	if len(errs) > 0 {
		return fmt.Errorf("errors closing models: %v", errs)
	}
	return nil
}

// ModelLoader is a function that loads a model from a path
type ModelLoader func(ctx context.Context, path string, metadata ModelMetadata) (Model, error)

var modelLoaders = make(map<ModelType]ModelLoader)

// RegisterModelLoader registers a model loader for a specific model type
func RegisterModelLoader(modelType ModelType, loader ModelLoader) {
	modelLoaders[modelType] = loader
}

// LoadModel loads a model using the appropriate loader
func LoadModel(ctx context.Context, modelType ModelType, path string, metadata ModelMetadata) (Model, error) {
	loader, exists := modelLoaders[modelType]
	if !exists {
		return nil, fmt.Errorf("no loader registered for model type: %s", modelType)
	}
	return loader(ctx, path, metadata)
}
