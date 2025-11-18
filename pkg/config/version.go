package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ConfigVersion represents a configuration file version
type ConfigVersion struct {
	Major int
	Minor int
}

// ParseVersion parses a version string (e.g., "v1", "v2.1")
func ParseVersion(version string) (ConfigVersion, error) {
	// Remove 'v' prefix if present
	version = strings.TrimPrefix(version, "v")

	parts := strings.Split(version, ".")

	if len(parts) == 0 || len(parts) > 2 {
		return ConfigVersion{}, fmt.Errorf("invalid version format: %s", version)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return ConfigVersion{}, fmt.Errorf("invalid major version: %w", err)
	}

	minor := 0
	if len(parts) == 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return ConfigVersion{}, fmt.Errorf("invalid minor version: %w", err)
		}
	}

	return ConfigVersion{
		Major: major,
		Minor: minor,
	}, nil
}

// String returns the string representation of the version
func (v ConfigVersion) String() string {
	if v.Minor == 0 {
		return fmt.Sprintf("v%d", v.Major)
	}
	return fmt.Sprintf("v%d.%d", v.Major, v.Minor)
}

// IsCompatible checks if a configuration version is compatible with the current version
func (v ConfigVersion) IsCompatible(other ConfigVersion) bool {
	// Major version must match
	if v.Major != other.Major {
		return false
	}

	// Minor version can be different within the same major version
	return true
}

// IsNewerThan checks if this version is newer than another
func (v ConfigVersion) IsNewerThan(other ConfigVersion) bool {
	if v.Major > other.Major {
		return true
	}
	if v.Major == other.Major && v.Minor > other.Minor {
		return true
	}
	return false
}

// GetCurrentVersion returns the current configuration version
func GetCurrentVersion() ConfigVersion {
	v, _ := ParseVersion(CurrentConfigVersion)
	return v
}

// ValidateVersion validates a configuration version
func ValidateVersion(config *Config) error {
	if config.Version == "" {
		return fmt.Errorf("configuration version is missing")
	}

	configVersion, err := ParseVersion(config.Version)
	if err != nil {
		return fmt.Errorf("invalid configuration version: %w", err)
	}

	currentVersion := GetCurrentVersion()

	if !currentVersion.IsCompatible(configVersion) {
		return fmt.Errorf("incompatible configuration version: %s (current: %s)",
			configVersion.String(), currentVersion.String())
	}

	if configVersion.IsNewerThan(currentVersion) {
		return fmt.Errorf("configuration version %s is newer than current version %s (upgrade required)",
			configVersion.String(), currentVersion.String())
	}

	return nil
}

// MigrateConfig migrates a configuration from an older version to the current version
func MigrateConfig(config *Config) (*Config, error) {
	if config.Version == "" {
		return nil, fmt.Errorf("configuration version is missing")
	}

	configVersion, err := ParseVersion(config.Version)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration version: %w", err)
	}

	currentVersion := GetCurrentVersion()

	// No migration needed if versions match
	if configVersion.Major == currentVersion.Major && configVersion.Minor == currentVersion.Minor {
		return config, nil
	}

	// Check if migration is possible
	if !currentVersion.IsCompatible(configVersion) {
		return nil, fmt.Errorf("cannot migrate from version %s to %s (major version mismatch)",
			configVersion.String(), currentVersion.String())
	}

	// Perform migration
	migrated := copyConfig(config)

	// Apply migrations for each minor version increment
	// For now, we only have v1, so no migrations are needed
	// Future migrations would be added here, e.g.:
	// if configVersion.Minor < 1 {
	//     migrateV1_0ToV1_1(migrated)
	// }

	// Update version
	migrated.Version = CurrentConfigVersion

	return migrated, nil
}

// VersionInfo holds version information
type VersionInfo struct {
	ConfigVersion   string   `json:"config_version"`
	SupportedVersions []string `json:"supported_versions"`
	MigrationPath   []string `json:"migration_path"`
}

// GetVersionInfo returns information about configuration versions
func GetVersionInfo() VersionInfo {
	return VersionInfo{
		ConfigVersion: CurrentConfigVersion,
		SupportedVersions: []string{
			"v1",
		},
		MigrationPath: []string{
			// Future migration paths will be listed here
		},
	}
}

// Example migration function (for future use)
// func migrateV1_0ToV1_1(config *Config) {
//     // Add new fields with default values
//     // Transform deprecated fields
//     // etc.
// }
