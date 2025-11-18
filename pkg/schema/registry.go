package schema

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/riferrei/srclient"
	"go.uber.org/zap"
)

// schemaRegistry implements the RegistryClient interface
type schemaRegistry struct {
	client *srclient.SchemaRegistryClient
	config *Config
	logger *zap.Logger

	// Cache for schemas
	cache      map[int]*cachedSchema
	cacheMutex sync.RWMutex

	// Metrics
	stats struct {
		sync.RWMutex
		cacheHits   int64
		cacheMisses int64
		errors      int64
		requests    int64
	}
}

// cachedSchema represents a cached schema with TTL
type cachedSchema struct {
	metadata  *SchemaMetadata
	expiresAt time.Time
}

// NewRegistryClient creates a new schema registry client
func NewRegistryClient(config *Config, logger *zap.Logger) (RegistryClient, error) {
	if config.RegistryURL == "" {
		return nil, errors.New("registry URL is required")
	}

	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.CacheSize == 0 {
		config.CacheSize = 1000
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}

	// Create srclient
	var client *srclient.SchemaRegistryClient
	var err error

	if config.Username != "" && config.Password != "" {
		client = srclient.CreateSchemaRegistryClient(config.RegistryURL)
		client.SetCredentials(config.Username, config.Password)
	} else {
		client = srclient.CreateSchemaRegistryClient(config.RegistryURL)
	}

	// Configure TLS if enabled
	if config.TLSEnabled {
		tlsConfig, err := createTLSConfig(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		client.SetTLSConfig(tlsConfig)
	}

	// Set timeout
	client.SetTimeout(config.Timeout)

	registry := &schemaRegistry{
		client: client,
		config: config,
		logger: logger,
	}

	if config.CacheEnabled {
		registry.cache = make(map[int]*cachedSchema, config.CacheSize)
		// Start cache cleanup routine
		go registry.cleanupCache()
	}

	logger.Info("Schema registry client initialized",
		zap.String("url", config.RegistryURL),
		zap.Bool("cache_enabled", config.CacheEnabled),
		zap.Int("cache_size", config.CacheSize),
	)

	return registry, nil
}

// createTLSConfig creates a TLS configuration from the config
func createTLSConfig(config *Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.TLSSkipVerify,
	}

	// Load CA certificate if provided
	if config.TLSCAPath != "" {
		caCert, err := os.ReadFile(config.TLSCAPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, errors.New("failed to append CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
	}

	// Load client certificate if provided
	if config.TLSCertPath != "" && config.TLSKeyPath != "" {
		cert, err := tls.LoadX509KeyPair(config.TLSCertPath, config.TLSKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificate: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

// GetSchema retrieves a schema by ID
func (r *schemaRegistry) GetSchema(ctx context.Context, schemaID int) (*SchemaMetadata, error) {
	r.recordRequest()

	// Check cache first
	if r.config.CacheEnabled {
		if cached := r.getFromCache(schemaID); cached != nil {
			r.recordCacheHit()
			return cached, nil
		}
		r.recordCacheMiss()
	}

	schema, err := r.client.GetSchema(schemaID)
	if err != nil {
		r.recordError()
		return nil, fmt.Errorf("failed to get schema %d: %w", schemaID, err)
	}

	metadata := &SchemaMetadata{
		ID:         schemaID,
		Schema:     schema.Schema(),
		SchemaType: SchemaType(schema.SchemaType()),
	}

	// Cache the result
	if r.config.CacheEnabled {
		r.addToCache(schemaID, metadata)
	}

	return metadata, nil
}

// GetLatestSchema retrieves the latest version of a schema for a subject
func (r *schemaRegistry) GetLatestSchema(ctx context.Context, subject string) (*SchemaMetadata, error) {
	r.recordRequest()

	schema, err := r.client.GetLatestSchema(subject)
	if err != nil {
		r.recordError()
		return nil, fmt.Errorf("failed to get latest schema for subject %s: %w", subject, err)
	}

	metadata := &SchemaMetadata{
		ID:         schema.ID(),
		Version:    schema.Version(),
		Schema:     schema.Schema(),
		Subject:    subject,
		SchemaType: SchemaType(schema.SchemaType()),
	}

	// Cache the result
	if r.config.CacheEnabled {
		r.addToCache(schema.ID(), metadata)
	}

	return metadata, nil
}

// GetSchemaByVersion retrieves a specific version of a schema
func (r *schemaRegistry) GetSchemaByVersion(ctx context.Context, subject string, version int) (*SchemaMetadata, error) {
	r.recordRequest()

	schema, err := r.client.GetSchemaByVersion(subject, version)
	if err != nil {
		r.recordError()
		return nil, fmt.Errorf("failed to get schema for subject %s version %d: %w", subject, version, err)
	}

	metadata := &SchemaMetadata{
		ID:         schema.ID(),
		Version:    schema.Version(),
		Schema:     schema.Schema(),
		Subject:    subject,
		SchemaType: SchemaType(schema.SchemaType()),
	}

	// Cache the result
	if r.config.CacheEnabled {
		r.addToCache(schema.ID(), metadata)
	}

	return metadata, nil
}

// RegisterSchema registers a new schema or returns existing if identical
func (r *schemaRegistry) RegisterSchema(ctx context.Context, subject string, schema string, schemaType SchemaType, references []Reference) (*SchemaMetadata, error) {
	r.recordRequest()

	// Convert references
	var refs []srclient.Reference
	for _, ref := range references {
		refs = append(refs, srclient.Reference{
			Name:    ref.Name,
			Subject: ref.Subject,
			Version: ref.Version,
		})
	}

	schemaObj, err := r.client.CreateSchema(subject, schema, srclient.SchemaType(schemaType), refs...)
	if err != nil {
		r.recordError()
		return nil, fmt.Errorf("failed to register schema for subject %s: %w", subject, err)
	}

	metadata := &SchemaMetadata{
		ID:         schemaObj.ID(),
		Version:    schemaObj.Version(),
		Schema:     schema,
		Subject:    subject,
		SchemaType: schemaType,
		References: references,
	}

	// Cache the result
	if r.config.CacheEnabled {
		r.addToCache(schemaObj.ID(), metadata)
	}

	r.logger.Info("Schema registered",
		zap.String("subject", subject),
		zap.Int("id", schemaObj.ID()),
		zap.Int("version", schemaObj.Version()),
	)

	return metadata, nil
}

// DeleteSubject deletes a subject and all its versions
func (r *schemaRegistry) DeleteSubject(ctx context.Context, subject string, permanent bool) error {
	r.recordRequest()

	err := r.client.DeleteSubject(subject, permanent)
	if err != nil {
		r.recordError()
		return fmt.Errorf("failed to delete subject %s: %w", subject, err)
	}

	r.logger.Info("Subject deleted",
		zap.String("subject", subject),
		zap.Bool("permanent", permanent),
	)

	return nil
}

// DeleteSchemaVersion deletes a specific version
func (r *schemaRegistry) DeleteSchemaVersion(ctx context.Context, subject string, version int, permanent bool) error {
	r.recordRequest()

	err := r.client.DeleteSubjectByVersion(subject, version, permanent)
	if err != nil {
		r.recordError()
		return fmt.Errorf("failed to delete subject %s version %d: %w", subject, version, err)
	}

	r.logger.Info("Schema version deleted",
		zap.String("subject", subject),
		zap.Int("version", version),
		zap.Bool("permanent", permanent),
	)

	return nil
}

// GetCompatibility gets the compatibility mode for a subject
func (r *schemaRegistry) GetCompatibility(ctx context.Context, subject string) (CompatibilityMode, error) {
	r.recordRequest()

	compat, err := r.client.GetCompatibility(subject)
	if err != nil {
		r.recordError()
		return "", fmt.Errorf("failed to get compatibility for subject %s: %w", subject, err)
	}

	return CompatibilityMode(compat), nil
}

// SetCompatibility sets the compatibility mode for a subject
func (r *schemaRegistry) SetCompatibility(ctx context.Context, subject string, mode CompatibilityMode) error {
	r.recordRequest()

	_, err := r.client.SetCompatibility(subject, srclient.Compatibility(mode))
	if err != nil {
		r.recordError()
		return fmt.Errorf("failed to set compatibility for subject %s: %w", subject, err)
	}

	r.logger.Info("Compatibility mode set",
		zap.String("subject", subject),
		zap.String("mode", string(mode)),
	)

	return nil
}

// TestCompatibility tests if a schema is compatible
func (r *schemaRegistry) TestCompatibility(ctx context.Context, subject string, schema string, schemaType SchemaType, version int) (bool, error) {
	r.recordRequest()

	compatible, err := r.client.IsSchemaCompatible(subject, schema, string(schemaType), version)
	if err != nil {
		r.recordError()
		return false, fmt.Errorf("failed to test compatibility for subject %s: %w", subject, err)
	}

	return compatible, nil
}

// GetSubjects lists all subjects
func (r *schemaRegistry) GetSubjects(ctx context.Context) ([]string, error) {
	r.recordRequest()

	subjects, err := r.client.GetSubjects()
	if err != nil {
		r.recordError()
		return nil, fmt.Errorf("failed to get subjects: %w", err)
	}

	return subjects, nil
}

// GetVersions lists all versions for a subject
func (r *schemaRegistry) GetVersions(ctx context.Context, subject string) ([]int, error) {
	r.recordRequest()

	versions, err := r.client.GetVersions(subject)
	if err != nil {
		r.recordError()
		return nil, fmt.Errorf("failed to get versions for subject %s: %w", subject, err)
	}

	return versions, nil
}

// Close closes the registry client
func (r *schemaRegistry) Close() error {
	r.logger.Info("Closing schema registry client",
		zap.Int64("cache_hits", r.getCacheHits()),
		zap.Int64("cache_misses", r.getCacheMisses()),
		zap.Int64("errors", r.getErrors()),
		zap.Int64("requests", r.getRequests()),
	)
	return nil
}

// Cache management methods

func (r *schemaRegistry) getFromCache(schemaID int) *SchemaMetadata {
	r.cacheMutex.RLock()
	defer r.cacheMutex.RUnlock()

	cached, exists := r.cache[schemaID]
	if !exists {
		return nil
	}

	if time.Now().After(cached.expiresAt) {
		return nil
	}

	return cached.metadata
}

func (r *schemaRegistry) addToCache(schemaID int, metadata *SchemaMetadata) {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	// Check cache size limit
	if len(r.cache) >= r.config.CacheSize {
		// Remove expired entries first
		r.evictExpired()

		// If still over limit, remove oldest entry
		if len(r.cache) >= r.config.CacheSize {
			for id := range r.cache {
				delete(r.cache, id)
				break
			}
		}
	}

	r.cache[schemaID] = &cachedSchema{
		metadata:  metadata,
		expiresAt: time.Now().Add(r.config.CacheTTL),
	}
}

func (r *schemaRegistry) evictExpired() {
	now := time.Now()
	for id, cached := range r.cache {
		if now.After(cached.expiresAt) {
			delete(r.cache, id)
		}
	}
}

func (r *schemaRegistry) cleanupCache() {
	ticker := time.NewTicker(r.config.CacheTTL / 2)
	defer ticker.Stop()

	for range ticker.C {
		r.cacheMutex.Lock()
		r.evictExpired()
		r.cacheMutex.Unlock()
	}
}

// Metrics methods

func (r *schemaRegistry) recordRequest() {
	r.stats.Lock()
	r.stats.requests++
	r.stats.Unlock()
}

func (r *schemaRegistry) recordError() {
	r.stats.Lock()
	r.stats.errors++
	r.stats.Unlock()
}

func (r *schemaRegistry) recordCacheHit() {
	r.stats.Lock()
	r.stats.cacheHits++
	r.stats.Unlock()
}

func (r *schemaRegistry) recordCacheMiss() {
	r.stats.Lock()
	r.stats.cacheMisses++
	r.stats.Unlock()
}

func (r *schemaRegistry) getCacheHits() int64 {
	r.stats.RLock()
	defer r.stats.RUnlock()
	return r.stats.cacheHits
}

func (r *schemaRegistry) getCacheMisses() int64 {
	r.stats.RLock()
	defer r.stats.RUnlock()
	return r.stats.cacheMisses
}

func (r *schemaRegistry) getErrors() int64 {
	r.stats.RLock()
	defer r.stats.RUnlock()
	return r.stats.errors
}

func (r *schemaRegistry) getRequests() int64 {
	r.stats.RLock()
	defer r.stats.RUnlock()
	return r.stats.requests
}

// GetStats returns current registry statistics
func (r *schemaRegistry) GetStats() map[string]int64 {
	return map[string]int64{
		"cache_hits":   r.getCacheHits(),
		"cache_misses": r.getCacheMisses(),
		"errors":       r.getErrors(),
		"requests":     r.getRequests(),
	}
}
