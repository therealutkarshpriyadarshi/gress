package ml

import (
	"context"
	"fmt"
	"os"
	"time"

	tflite "github.com/mattn/go-tflite"
)

// TensorFlowModel wraps a TensorFlow Lite model for inference
type TensorFlowModel struct {
	model       *tflite.Model
	interpreter *tflite.Interpreter
	metadata    ModelMetadata
	inputNames  []string
	outputNames []string
}

// NewTensorFlowModel creates a new TensorFlow Lite model from a file
func NewTensorFlowModel(ctx context.Context, path string, metadata ModelMetadata) (*TensorFlowModel, error) {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("model file not found: %s", path)
	}

	// Load the TFLite model
	model := tflite.NewModelFromFile(path)
	if model == nil {
		return nil, fmt.Errorf("failed to load TensorFlow Lite model from %s", path)
	}

	// Create interpreter options
	options := tflite.NewInterpreterOptions()
	options.SetNumThread(4) // Use 4 threads for inference
	defer options.Delete()

	// Create interpreter
	interpreter := tflite.NewInterpreter(model, options)
	if interpreter == nil {
		model.Delete()
		return nil, fmt.Errorf("failed to create TensorFlow Lite interpreter")
	}

	// Allocate tensors
	if status := interpreter.AllocateTensors(); status != tflite.OK {
		interpreter.Delete()
		model.Delete()
		return nil, fmt.Errorf("failed to allocate tensors: status %d", status)
	}

	// Get input and output tensor information
	inputCount := interpreter.GetInputTensorCount()
	outputCount := interpreter.GetOutputTensorCount()

	inputNames := make([]string, inputCount)
	for i := 0; i < inputCount; i++ {
		tensor := interpreter.GetInputTensor(i)
		inputNames[i] = tensor.Name()
	}

	outputNames := make([]string, outputCount)
	for i := 0; i < outputCount; i++ {
		tensor := interpreter.GetOutputTensor(i)
		outputNames[i] = tensor.Name()
	}

	// Update metadata
	metadata.InputNames = inputNames
	metadata.OutputNames = outputNames
	metadata.Type = ModelTypeTensorFlow
	if metadata.UpdatedAt.IsZero() {
		metadata.UpdatedAt = time.Now()
	}

	return &TensorFlowModel{
		model:       model,
		interpreter: interpreter,
		metadata:    metadata,
		inputNames:  inputNames,
		outputNames: outputNames,
	}, nil
}

// Predict performs inference on a single feature set
func (m *TensorFlowModel) Predict(ctx context.Context, features map[string]interface{}) (*PredictionResponse, error) {
	start := time.Now()

	// Set input tensors
	inputCount := m.interpreter.GetInputTensorCount()
	for i := 0; i < inputCount; i++ {
		tensor := m.interpreter.GetInputTensor(i)
		name := m.inputNames[i]

		value, exists := features[name]
		if !exists {
			// Try using index if name not found
			indexKey := fmt.Sprintf("input_%d", i)
			value, exists = features[indexKey]
			if !exists {
				return nil, fmt.Errorf("missing input feature: %s (or %s)", name, indexKey)
			}
		}

		if err := m.setTensorData(tensor, value); err != nil {
			return nil, fmt.Errorf("failed to set input tensor %s: %w", name, err)
		}
	}

	// Run inference
	if status := m.interpreter.Invoke(); status != tflite.OK {
		return nil, fmt.Errorf("inference failed with status: %d", status)
	}

	// Get output tensors
	predictions := make(map[string]interface{})
	outputCount := m.interpreter.GetOutputTensorCount()
	for i := 0; i < outputCount; i++ {
		tensor := m.interpreter.GetOutputTensor(i)
		name := m.outputNames[i]

		output, err := m.getTensorData(tensor)
		if err != nil {
			return nil, fmt.Errorf("failed to get output tensor %s: %w", name, err)
		}
		predictions[name] = output
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
func (m *TensorFlowModel) PredictBatch(ctx context.Context, requests []*PredictionRequest) ([]*PredictionResponse, error) {
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
func (m *TensorFlowModel) GetMetadata() ModelMetadata {
	return m.metadata
}

// Warmup performs model warmup
func (m *TensorFlowModel) Warmup(ctx context.Context) error {
	// Create dummy input based on tensor shapes
	dummyFeatures := make(map[string]interface{})

	inputCount := m.interpreter.GetInputTensorCount()
	for i := 0; i < inputCount; i++ {
		tensor := m.interpreter.GetInputTensor(i)
		name := m.inputNames[i]

		// Get tensor type and shape
		tensorType := tensor.Type()
		byteSize := tensor.ByteSize()

		// Create dummy data based on type
		switch tensorType {
		case tflite.Float32:
			size := byteSize / 4 // 4 bytes per float32
			dummyFeatures[name] = make([]float32, size)
		case tflite.Int32:
			size := byteSize / 4
			dummyFeatures[name] = make([]int32, size)
		case tflite.Int64:
			size := byteSize / 8
			dummyFeatures[name] = make([]int64, size)
		case tflite.UInt8:
			dummyFeatures[name] = make([]uint8, byteSize)
		default:
			return fmt.Errorf("unsupported tensor type for warmup: %v", tensorType)
		}
	}

	// Run a warmup prediction
	_, err := m.Predict(ctx, dummyFeatures)
	return err
}

// Close releases model resources
func (m *TensorFlowModel) Close() error {
	if m.interpreter != nil {
		m.interpreter.Delete()
	}
	if m.model != nil {
		m.model.Delete()
	}
	return nil
}

// setTensorData sets tensor data from a feature value
func (m *TensorFlowModel) setTensorData(tensor *tflite.Tensor, value interface{}) error {
	tensorType := tensor.Type()

	switch tensorType {
	case tflite.Float32:
		switch v := value.(type) {
		case []float32:
			tensor.SetFloat32s(v)
		case []float64:
			// Convert float64 to float32
			f32 := make([]float32, len(v))
			for i, val := range v {
				f32[i] = float32(val)
			}
			tensor.SetFloat32s(f32)
		case float32:
			tensor.SetFloat32s([]float32{v})
		case float64:
			tensor.SetFloat32s([]float32{float32(v)})
		default:
			return fmt.Errorf("cannot convert %T to float32 tensor", value)
		}

	case tflite.Int32:
		switch v := value.(type) {
		case []int32:
			tensor.SetInt32s(v)
		case []int:
			i32 := make([]int32, len(v))
			for i, val := range v {
				i32[i] = int32(val)
			}
			tensor.SetInt32s(i32)
		case int32:
			tensor.SetInt32s([]int32{v})
		case int:
			tensor.SetInt32s([]int32{int32(v)})
		default:
			return fmt.Errorf("cannot convert %T to int32 tensor", value)
		}

	case tflite.Int64:
		switch v := value.(type) {
		case []int64:
			tensor.SetInt64s(v)
		case []int:
			i64 := make([]int64, len(v))
			for i, val := range v {
				i64[i] = int64(val)
			}
			tensor.SetInt64s(i64)
		case int64:
			tensor.SetInt64s([]int64{v})
		case int:
			tensor.SetInt64s([]int64{int64(v)})
		default:
			return fmt.Errorf("cannot convert %T to int64 tensor", value)
		}

	case tflite.UInt8:
		switch v := value.(type) {
		case []uint8:
			tensor.SetUint8s(v)
		case uint8:
			tensor.SetUint8s([]uint8{v})
		default:
			return fmt.Errorf("cannot convert %T to uint8 tensor", value)
		}

	default:
		return fmt.Errorf("unsupported tensor type: %v", tensorType)
	}

	return nil
}

// getTensorData gets tensor data as a feature value
func (m *TensorFlowModel) getTensorData(tensor *tflite.Tensor) (interface{}, error) {
	tensorType := tensor.Type()

	switch tensorType {
	case tflite.Float32:
		return tensor.Float32s(), nil
	case tflite.Int32:
		return tensor.Int32s(), nil
	case tflite.Int64:
		return tensor.Int64s(), nil
	case tflite.UInt8:
		return tensor.UInt8s(), nil
	default:
		return nil, fmt.Errorf("unsupported tensor type: %v", tensorType)
	}
}

// init registers the TensorFlow Lite model loader
func init() {
	RegisterModelLoader(ModelTypeTensorFlow, func(ctx context.Context, path string, metadata ModelMetadata) (Model, error) {
		return NewTensorFlowModel(ctx, path, metadata)
	})
}
