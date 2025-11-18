package ml

import (
	"context"
	"fmt"
	"os"
	"time"

	ort "github.com/yalue/onnxruntime_go"
)

// ONNXModel wraps an ONNX model for inference
type ONNXModel struct {
	session      *ort.AdvancedSession
	metadata     ModelMetadata
	inputNames   []string
	outputNames  []string
	inputShapes  []ort.Shape
	outputShapes []ort.Shape
}

// NewONNXModel creates a new ONNX model from a file
func NewONNXModel(ctx context.Context, path string, metadata ModelMetadata) (*ONNXModel, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("model file not found: %s", path)
	}

	// Create ONNX runtime session
	session, err := ort.NewAdvancedSession(path,
		[]string{}, // Input names (will be auto-detected)
		[]string{}, // Output names (will be auto-detected)
		nil,        // Destination tensors (will be created)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ONNX session: %w", err)
	}

	// Get input and output information
	inputCount := session.GetInputCount()
	outputCount := session.GetOutputCount()

	inputNames := make([]string, inputCount)
	inputShapes := make([]ort.Shape, inputCount)
	for i := 0; i < inputCount; i++ {
		inputNames[i] = session.GetInputName(i)
		inputShapes[i] = session.GetInputTypeInfo(i).GetShape()
	}

	outputNames := make([]string, outputCount)
	outputShapes := make([]ort.Shape, outputCount)
	for i := 0; i < outputCount; i++ {
		outputNames[i] = session.GetOutputName(i)
		outputShapes[i] = session.GetOutputTypeInfo(i).GetShape()
	}

	// Update metadata with input/output names
	metadata.InputNames = inputNames
	metadata.OutputNames = outputNames
	metadata.Type = ModelTypeONNX
	if metadata.UpdatedAt.IsZero() {
		metadata.UpdatedAt = time.Now()
	}

	model := &ONNXModel{
		session:      session,
		metadata:     metadata,
		inputNames:   inputNames,
		outputNames:  outputNames,
		inputShapes:  inputShapes,
		outputShapes: outputShapes,
	}

	return model, nil
}

// Predict performs inference on a single feature set
func (m *ONNXModel) Predict(ctx context.Context, features map[string]interface{}) (*PredictionResponse, error) {
	start := time.Now()

	// Prepare input tensors
	inputTensors := make([]ort.Value, len(m.inputNames))
	for i, name := range m.inputNames {
		value, exists := features[name]
		if !exists {
			return nil, fmt.Errorf("missing input feature: %s", name)
		}

		tensor, err := m.featureToTensor(value, m.inputShapes[i])
		if err != nil {
			return nil, fmt.Errorf("failed to convert feature %s to tensor: %w", name, err)
		}
		inputTensors[i] = tensor
	}

	// Create output tensors
	outputTensors := make([]ort.Value, len(m.outputNames))
	for i := range m.outputNames {
		tensor, err := ort.NewEmptyTensor[float32](m.outputShapes[i])
		if err != nil {
			return nil, fmt.Errorf("failed to create output tensor: %w", err)
		}
		outputTensors[i] = tensor
	}

	// Run inference
	if err := m.session.Run(inputTensors, outputTensors); err != nil {
		// Clean up tensors
		for _, t := range inputTensors {
			t.Destroy()
		}
		for _, t := range outputTensors {
			t.Destroy()
		}
		return nil, fmt.Errorf("inference failed: %w", err)
	}

	// Extract outputs
	predictions := make(map[string]interface{})
	for i, name := range m.outputNames {
		output, err := m.tensorToFeature(outputTensors[i])
		if err != nil {
			// Clean up tensors
			for _, t := range inputTensors {
				t.Destroy()
			}
			for _, t := range outputTensors {
				t.Destroy()
			}
			return nil, fmt.Errorf("failed to extract output %s: %w", name, err)
		}
		predictions[name] = output
	}

	// Clean up tensors
	for _, t := range inputTensors {
		t.Destroy()
	}
	for _, t := range outputTensors {
		t.Destroy()
	}

	latency := time.Since(start)

	// Determine primary prediction (first output)
	var primaryPrediction interface{}
	if len(m.outputNames) > 0 {
		primaryPrediction = predictions[m.outputNames[0]]
	}

	return &PredictionResponse{
		Prediction: primaryPrediction,
		Metadata:   predictions,
		Latency:    latency,
		ModelInfo:  m.metadata,
	}, nil
}

// PredictBatch performs batch inference
func (m *ONNXModel) PredictBatch(ctx context.Context, requests []*PredictionRequest) ([]*PredictionResponse, error) {
	responses := make([]*PredictionResponse, len(requests))

	for i, req := range requests {
		resp, err := m.Predict(ctx, req.Features)
		if err != nil {
			return nil, fmt.Errorf("failed to predict batch item %d: %w", i, err)
		}
		responses[i] = resp
	}

	return responses, nil
}

// GetMetadata returns model metadata
func (m *ONNXModel) GetMetadata() ModelMetadata {
	return m.metadata
}

// Warmup performs model warmup
func (m *ONNXModel) Warmup(ctx context.Context) error {
	// Create dummy input
	dummyFeatures := make(map[string]interface{})
	for i, name := range m.inputNames {
		// Create a dummy tensor with the expected shape
		shape := m.inputShapes[i]
		size := 1
		for _, dim := range shape.Dimensions {
			if dim > 0 {
				size *= int(dim)
			}
		}
		dummyData := make([]float32, size)
		dummyFeatures[name] = dummyData
	}

	// Run a warmup prediction
	_, err := m.Predict(ctx, dummyFeatures)
	return err
}

// Close releases model resources
func (m *ONNXModel) Close() error {
	if m.session != nil {
		return m.session.Destroy()
	}
	return nil
}

// featureToTensor converts a feature value to an ONNX tensor
func (m *ONNXModel) featureToTensor(value interface{}, shape ort.Shape) (ort.Value, error) {
	// Handle different input types
	switch v := value.(type) {
	case []float32:
		return ort.NewTensor(shape, v)
	case []float64:
		// Convert float64 to float32
		f32 := make([]float32, len(v))
		for i, val := range v {
			f32[i] = float32(val)
		}
		return ort.NewTensor(shape, f32)
	case []int64:
		return ort.NewTensor(shape, v)
	case []int32:
		return ort.NewTensor(shape, v)
	case []int:
		// Convert int to int64
		i64 := make([]int64, len(v))
		for i, val := range v {
			i64[i] = int64(val)
		}
		return ort.NewTensor(shape, i64)
	default:
		return nil, fmt.Errorf("unsupported feature type: %T", value)
	}
}

// tensorToFeature converts an ONNX tensor to a feature value
func (m *ONNXModel) tensorToFeature(tensor ort.Value) (interface{}, error) {
	// Try to extract as float32 first (most common)
	if data := tensor.GetData(); data != nil {
		switch v := data.(type) {
		case []float32:
			return v, nil
		case []float64:
			return v, nil
		case []int64:
			return v, nil
		case []int32:
			return v, nil
		default:
			return v, nil
		}
	}
	return nil, fmt.Errorf("failed to extract tensor data")
}

// init registers the ONNX model loader
func init() {
	// Initialize ONNX Runtime
	if err := ort.InitializeEnvironment(); err != nil {
		// Note: This is called at package initialization, so we can't return the error
		// In production, you might want to handle this differently
		panic(fmt.Sprintf("failed to initialize ONNX Runtime: %v", err))
	}

	RegisterModelLoader(ModelTypeONNX, func(ctx context.Context, path string, metadata ModelMetadata) (Model, error) {
		return NewONNXModel(ctx, path, metadata)
	})
}
