package metrics

import (
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
)

// RuntimeCollector collects Go runtime metrics
type RuntimeCollector struct {
	// Memory metrics
	allocBytes       prometheus.Gauge
	totalAllocBytes  prometheus.Counter
	sysBytes         prometheus.Gauge
	numGC            prometheus.Counter
	gcPauseSeconds   prometheus.Histogram
	nextGCBytes      prometheus.Gauge
	heapObjects      prometheus.Gauge
	heapInuseBytes   prometheus.Gauge
	heapIdleBytes    prometheus.Gauge
	heapReleasedBytes prometheus.Gauge

	// Goroutine metrics
	numGoroutines prometheus.Gauge

	// CPU metrics
	numCPU        prometheus.Gauge
	numCgoCall    prometheus.Counter

	logger *zap.Logger
	stopCh chan struct{}
}

// NewRuntimeCollector creates a new runtime metrics collector
func NewRuntimeCollector(registry *prometheus.Registry, logger *zap.Logger) *RuntimeCollector {
	rc := &RuntimeCollector{
		allocBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_alloc_bytes",
			Help: "Bytes of allocated heap objects",
		}),
		totalAllocBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gress_runtime_total_alloc_bytes_total",
			Help: "Cumulative bytes allocated for heap objects",
		}),
		sysBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_sys_bytes",
			Help: "Total bytes of memory obtained from the OS",
		}),
		numGC: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gress_runtime_gc_total",
			Help: "Number of completed GC cycles",
		}),
		gcPauseSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "gress_runtime_gc_pause_seconds",
			Help:    "GC pause duration in seconds",
			Buckets: []float64{0.00001, 0.0001, 0.001, 0.01, 0.1, 1},
		}),
		nextGCBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_next_gc_bytes",
			Help: "Target heap size for next GC cycle",
		}),
		heapObjects: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_heap_objects",
			Help: "Number of allocated heap objects",
		}),
		heapInuseBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_heap_inuse_bytes",
			Help: "Bytes in in-use spans",
		}),
		heapIdleBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_heap_idle_bytes",
			Help: "Bytes in idle spans",
		}),
		heapReleasedBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_heap_released_bytes",
			Help: "Bytes of physical memory returned to the OS",
		}),
		numGoroutines: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_goroutines",
			Help: "Number of goroutines that currently exist",
		}),
		numCPU: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_runtime_cpu_cores",
			Help: "Number of logical CPU cores",
		}),
		numCgoCall: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "gress_runtime_cgo_calls_total",
			Help: "Number of cgo calls made",
		}),
		logger: logger,
		stopCh: make(chan struct{}),
	}

	// Register metrics
	registry.MustRegister(rc.allocBytes)
	registry.MustRegister(rc.totalAllocBytes)
	registry.MustRegister(rc.sysBytes)
	registry.MustRegister(rc.numGC)
	registry.MustRegister(rc.gcPauseSeconds)
	registry.MustRegister(rc.nextGCBytes)
	registry.MustRegister(rc.heapObjects)
	registry.MustRegister(rc.heapInuseBytes)
	registry.MustRegister(rc.heapIdleBytes)
	registry.MustRegister(rc.heapReleasedBytes)
	registry.MustRegister(rc.numGoroutines)
	registry.MustRegister(rc.numCPU)
	registry.MustRegister(rc.numCgoCall)

	return rc
}

// Start begins collecting runtime metrics
func (rc *RuntimeCollector) Start(interval time.Duration) {
	rc.logger.Info("Starting runtime metrics collection", zap.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rc.collect()
			case <-rc.stopCh:
				return
			}
		}
	}()
}

// Stop stops collecting runtime metrics
func (rc *RuntimeCollector) Stop() {
	rc.logger.Info("Stopping runtime metrics collection")
	close(rc.stopCh)
}

// collect gathers current runtime metrics
func (rc *RuntimeCollector) collect() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Memory metrics
	rc.allocBytes.Set(float64(m.Alloc))
	rc.totalAllocBytes.Add(float64(m.TotalAlloc))
	rc.sysBytes.Set(float64(m.Sys))
	rc.nextGCBytes.Set(float64(m.NextGC))
	rc.heapObjects.Set(float64(m.HeapObjects))
	rc.heapInuseBytes.Set(float64(m.HeapInuse))
	rc.heapIdleBytes.Set(float64(m.HeapIdle))
	rc.heapReleasedBytes.Set(float64(m.HeapReleased))

	// GC metrics
	rc.numGC.Add(float64(m.NumGC))

	// Record GC pause times
	if m.NumGC > 0 {
		// Get the most recent GC pause time
		lastPause := m.PauseNs[(m.NumGC+255)%256]
		rc.gcPauseSeconds.Observe(float64(lastPause) / 1e9)
	}

	// Goroutine metrics
	rc.numGoroutines.Set(float64(runtime.NumGoroutine()))

	// CPU metrics
	rc.numCPU.Set(float64(runtime.NumCPU()))
	rc.numCgoCall.Add(float64(runtime.NumCgoCall()))
}

// SystemCollector collects system-level metrics
type SystemCollector struct {
	// Process metrics
	uptimeSeconds    prometheus.Gauge
	startTimeSeconds prometheus.Gauge

	logger    *zap.Logger
	startTime time.Time
	stopCh    chan struct{}
}

// NewSystemCollector creates a new system metrics collector
func NewSystemCollector(registry *prometheus.Registry, logger *zap.Logger) *SystemCollector {
	startTime := time.Now()

	sc := &SystemCollector{
		uptimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_process_uptime_seconds",
			Help: "Time in seconds since the process started",
		}),
		startTimeSeconds: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "gress_process_start_time_seconds",
			Help: "Start time of the process since unix epoch in seconds",
		}),
		logger:    logger,
		startTime: startTime,
		stopCh:    make(chan struct{}),
	}

	// Register metrics
	registry.MustRegister(sc.uptimeSeconds)
	registry.MustRegister(sc.startTimeSeconds)

	// Set start time (this doesn't change)
	sc.startTimeSeconds.Set(float64(startTime.Unix()))

	return sc
}

// Start begins collecting system metrics
func (sc *SystemCollector) Start(interval time.Duration) {
	sc.logger.Info("Starting system metrics collection", zap.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sc.collect()
			case <-sc.stopCh:
				return
			}
		}
	}()
}

// Stop stops collecting system metrics
func (sc *SystemCollector) Stop() {
	sc.logger.Info("Stopping system metrics collection")
	close(sc.stopCh)
}

// collect gathers current system metrics
func (sc *SystemCollector) collect() {
	sc.uptimeSeconds.Set(time.Since(sc.startTime).Seconds())
}
