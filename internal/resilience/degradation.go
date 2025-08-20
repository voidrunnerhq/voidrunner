package resilience

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// DegradationLevel represents the level of service degradation
type DegradationLevel int

const (
	// LevelNormal represents normal operation with all features enabled
	LevelNormal DegradationLevel = iota
	
	// LevelLimited represents limited operation with some features disabled
	LevelLimited
	
	// LevelMinimal represents minimal operation with only essential features
	LevelMinimal
	
	// LevelEmergency represents emergency mode with severely limited functionality
	LevelEmergency
)

// String returns the string representation of the degradation level
func (level DegradationLevel) String() string {
	switch level {
	case LevelNormal:
		return "normal"
	case LevelLimited:
		return "limited"
	case LevelMinimal:
		return "minimal"
	case LevelEmergency:
		return "emergency"
	default:
		return "unknown"
	}
}

// FeatureName represents a feature that can be degraded
type FeatureName string

const (
	// FeatureLogging represents logging functionality
	FeatureLogging FeatureName = "logging"
	
	// FeatureMetrics represents metrics collection
	FeatureMetrics FeatureName = "metrics"
	
	// FeatureComplexTasks represents complex task execution
	FeatureComplexTasks FeatureName = "complex_tasks"
	
	// FeatureConcurrentExecution represents concurrent task execution
	FeatureConcurrentExecution FeatureName = "concurrent_execution"
	
	// FeatureResourceMonitoring represents resource monitoring
	FeatureResourceMonitoring FeatureName = "resource_monitoring"
	
	// FeatureAlerts represents alerting functionality
	FeatureAlerts FeatureName = "alerts"
	
	// FeatureDetailedReporting represents detailed reporting
	FeatureDetailedReporting FeatureName = "detailed_reporting"
)

// FeatureConfig defines configuration for a specific feature
type FeatureConfig struct {
	Name              FeatureName `json:"name"`
	EnabledAtLevel    DegradationLevel `json:"enabled_at_level"`
	ResourceWeight    float64     `json:"resource_weight"`    // How much resources this feature uses (0-1)
	CriticalityScore  float64     `json:"criticality_score"`  // How critical this feature is (0-1)
	FallbackBehavior  string      `json:"fallback_behavior"`  // What to do when disabled
}

// DegradationConfig holds configuration for graceful degradation
type DegradationConfig struct {
	// Thresholds for triggering different degradation levels
	LimitedModeThreshold   ResourceThreshold `json:"limited_mode_threshold"`
	MinimalModeThreshold   ResourceThreshold `json:"minimal_mode_threshold"`
	EmergencyModeThreshold ResourceThreshold `json:"emergency_mode_threshold"`
	
	// Feature configurations
	Features map[FeatureName]*FeatureConfig `json:"features"`
	
	// Recovery settings
	RecoveryInterval       time.Duration `json:"recovery_interval"`        // How often to check for recovery
	RecoveryStabilityTime  time.Duration `json:"recovery_stability_time"`  // How long to wait before upgrading level
	
	// Notification settings
	NotifyOnLevelChange bool `json:"notify_on_level_change"`
}

// ResourceThreshold defines thresholds for triggering degradation
type ResourceThreshold struct {
	CPUPercent       float64 `json:"cpu_percent"`
	MemoryPercent    float64 `json:"memory_percent"`
	ErrorRate        float64 `json:"error_rate"`
	ResponseTimeMs   int64   `json:"response_time_ms"`
	ActiveConnections int    `json:"active_connections"`
}

// DefaultDegradationConfig returns sensible defaults
func DefaultDegradationConfig() *DegradationConfig {
	features := map[FeatureName]*FeatureConfig{
		FeatureLogging: {
			Name:             FeatureLogging,
			EnabledAtLevel:   LevelNormal,
			ResourceWeight:   0.1,
			CriticalityScore: 0.3,
			FallbackBehavior: "basic_logging_only",
		},
		FeatureMetrics: {
			Name:             FeatureMetrics,
			EnabledAtLevel:   LevelNormal,
			ResourceWeight:   0.15,
			CriticalityScore: 0.4,
			FallbackBehavior: "disable_collection",
		},
		FeatureComplexTasks: {
			Name:             FeatureComplexTasks,
			EnabledAtLevel:   LevelLimited,
			ResourceWeight:   0.3,
			CriticalityScore: 0.6,
			FallbackBehavior: "simple_tasks_only",
		},
		FeatureConcurrentExecution: {
			Name:             FeatureConcurrentExecution,
			EnabledAtLevel:   LevelMinimal,
			ResourceWeight:   0.4,
			CriticalityScore: 0.8,
			FallbackBehavior: "sequential_execution",
		},
		FeatureResourceMonitoring: {
			Name:             FeatureResourceMonitoring,
			EnabledAtLevel:   LevelNormal,
			ResourceWeight:   0.2,
			CriticalityScore: 0.5,
			FallbackBehavior: "basic_monitoring",
		},
		FeatureAlerts: {
			Name:             FeatureAlerts,
			EnabledAtLevel:   LevelLimited,
			ResourceWeight:   0.05,
			CriticalityScore: 0.7,
			FallbackBehavior: "critical_alerts_only",
		},
		FeatureDetailedReporting: {
			Name:             FeatureDetailedReporting,
			EnabledAtLevel:   LevelNormal,
			ResourceWeight:   0.25,
			CriticalityScore: 0.2,
			FallbackBehavior: "basic_reports_only",
		},
	}

	return &DegradationConfig{
		LimitedModeThreshold: ResourceThreshold{
			CPUPercent:       75.0,
			MemoryPercent:    80.0,
			ErrorRate:        10.0,
			ResponseTimeMs:   2000,
			ActiveConnections: 800,
		},
		MinimalModeThreshold: ResourceThreshold{
			CPUPercent:       85.0,
			MemoryPercent:    90.0,
			ErrorRate:        20.0,
			ResponseTimeMs:   5000,
			ActiveConnections: 1000,
		},
		EmergencyModeThreshold: ResourceThreshold{
			CPUPercent:       95.0,
			MemoryPercent:    95.0,
			ErrorRate:        50.0,
			ResponseTimeMs:   10000,
			ActiveConnections: 1200,
		},
		Features:               features,
		RecoveryInterval:       30 * time.Second,
		RecoveryStabilityTime:  2 * time.Minute,
		NotifyOnLevelChange:    true,
	}
}

// DegradationStats holds statistics about degradation
type DegradationStats struct {
	CurrentLevel        DegradationLevel              `json:"current_level"`
	LevelChangedAt      time.Time                     `json:"level_changed_at"`
	TimeInCurrentLevel  time.Duration                 `json:"time_in_current_level"`
	EnabledFeatures     []FeatureName                 `json:"enabled_features"`
	DisabledFeatures    []FeatureName                 `json:"disabled_features"`
	ResourceSavings     float64                       `json:"resource_savings"`
	LevelChangeHistory  []LevelChangeEvent            `json:"level_change_history"`
	FeatureStates       map[FeatureName]bool          `json:"feature_states"`
}

// LevelChangeEvent represents a degradation level change event
type LevelChangeEvent struct {
	FromLevel   DegradationLevel `json:"from_level"`
	ToLevel     DegradationLevel `json:"to_level"`
	Timestamp   time.Time        `json:"timestamp"`
	Reason      string           `json:"reason"`
	Triggered   []string         `json:"triggered"`  // Which thresholds were exceeded
}

// GracefulDegradation manages graceful service degradation
type GracefulDegradation struct {
	mu                    sync.RWMutex
	config                *DegradationConfig
	logger                *slog.Logger
	metricsProvider       MetricsProvider
	
	// Current state
	currentLevel          DegradationLevel
	levelChangedAt        time.Time
	enabledFeatures       map[FeatureName]bool
	
	// History
	levelChangeHistory    []LevelChangeEvent
	
	// Monitoring
	ctx                   context.Context
	cancel                context.CancelFunc
	wg                    sync.WaitGroup
	
	// Recovery tracking
	lastStableTime        time.Time
	pendingUpgrade        *DegradationLevel
}

// NewGracefulDegradation creates a new graceful degradation manager
func NewGracefulDegradation(config *DegradationConfig, metricsProvider MetricsProvider, logger *slog.Logger) *GracefulDegradation {
	if config == nil {
		config = DefaultDegradationConfig()
	}
	
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	gd := &GracefulDegradation{
		config:             config,
		logger:             logger.With("component", "graceful_degradation"),
		metricsProvider:    metricsProvider,
		currentLevel:       LevelNormal,
		levelChangedAt:     time.Now(),
		enabledFeatures:    make(map[FeatureName]bool),
		levelChangeHistory: make([]LevelChangeEvent, 0),
		ctx:                ctx,
		cancel:             cancel,
		lastStableTime:     time.Now(),
	}

	// Initialize feature states based on normal level
	gd.updateFeatureStates()

	// Start monitoring if metrics provider is available
	if metricsProvider != nil {
		gd.startMonitoring()
	}

	return gd
}

// IsFeatureEnabled checks if a feature is currently enabled
func (gd *GracefulDegradation) IsFeatureEnabled(feature FeatureName) bool {
	gd.mu.RLock()
	defer gd.mu.RUnlock()
	
	enabled, exists := gd.enabledFeatures[feature]
	return exists && enabled
}

// GetCurrentLevel returns the current degradation level
func (gd *GracefulDegradation) GetCurrentLevel() DegradationLevel {
	gd.mu.RLock()
	defer gd.mu.RUnlock()
	return gd.currentLevel
}

// SetLevel manually sets the degradation level
func (gd *GracefulDegradation) SetLevel(level DegradationLevel, reason string) {
	gd.mu.Lock()
	defer gd.mu.Unlock()
	
	if level == gd.currentLevel {
		return
	}

	gd.changeLevel(level, reason, []string{"manual_override"})
}

// startMonitoring starts background monitoring for automatic degradation
func (gd *GracefulDegradation) startMonitoring() {
	gd.wg.Add(1)
	go func() {
		defer gd.wg.Done()
		ticker := time.NewTicker(gd.config.RecoveryInterval)
		defer ticker.Stop()

		for {
			select {
			case <-gd.ctx.Done():
				return
			case <-ticker.C:
				gd.evaluateResourcesAndAdjustLevel()
			}
		}
	}()
}

// evaluateResourcesAndAdjustLevel checks current resources and adjusts degradation level
func (gd *GracefulDegradation) evaluateResourcesAndAdjustLevel() {
	if gd.metricsProvider == nil {
		return
	}

	metrics := gd.metricsProvider.GetCurrentMetrics()
	if metrics == nil {
		return
	}

	gd.mu.Lock()
	defer gd.mu.Unlock()

	// Determine appropriate level based on current metrics
	targetLevel, triggeredThresholds := gd.determineTargetLevel(metrics)

	if targetLevel == gd.currentLevel {
		// No change needed, reset stability tracking
		gd.lastStableTime = time.Now()
		gd.pendingUpgrade = nil
		return
	}

	// If we want to upgrade (improve service level), check stability
	if targetLevel < gd.currentLevel {
		if gd.pendingUpgrade == nil || *gd.pendingUpgrade != targetLevel {
			// Start tracking this potential upgrade
			gd.pendingUpgrade = &targetLevel
			gd.lastStableTime = time.Now()
			gd.logger.Debug("potential service level upgrade detected, monitoring for stability",
				"current_level", gd.currentLevel.String(),
				"target_level", targetLevel.String())
			return
		}

		// Check if we've been stable long enough
		if time.Since(gd.lastStableTime) < gd.config.RecoveryStabilityTime {
			return
		}
	}

	// Make the level change
	reason := "resource_thresholds_triggered"
	if targetLevel < gd.currentLevel {
		reason = "resource_conditions_improved"
	}

	gd.changeLevel(targetLevel, reason, triggeredThresholds)
}

// determineTargetLevel determines the appropriate degradation level based on metrics
func (gd *GracefulDegradation) determineTargetLevel(metrics *SystemMetrics) (DegradationLevel, []string) {
	var triggeredThresholds []string

	// Check emergency mode
	if gd.exceedsThreshold(metrics, gd.config.EmergencyModeThreshold, &triggeredThresholds) {
		return LevelEmergency, triggeredThresholds
	}

	// Check minimal mode
	if gd.exceedsThreshold(metrics, gd.config.MinimalModeThreshold, &triggeredThresholds) {
		return LevelMinimal, triggeredThresholds
	}

	// Check limited mode
	if gd.exceedsThreshold(metrics, gd.config.LimitedModeThreshold, &triggeredThresholds) {
		return LevelLimited, triggeredThresholds
	}

	// All thresholds are fine, can operate normally
	return LevelNormal, nil
}

// exceedsThreshold checks if metrics exceed the given threshold
func (gd *GracefulDegradation) exceedsThreshold(metrics *SystemMetrics, threshold ResourceThreshold, triggered *[]string) bool {
	exceeded := false

	if metrics.CPUPercent >= threshold.CPUPercent {
		*triggered = append(*triggered, fmt.Sprintf("cpu_%.1f%%", metrics.CPUPercent))
		exceeded = true
	}
	if metrics.MemoryPercent >= threshold.MemoryPercent {
		*triggered = append(*triggered, fmt.Sprintf("memory_%.1f%%", metrics.MemoryPercent))
		exceeded = true
	}
	if metrics.ErrorRate >= threshold.ErrorRate {
		*triggered = append(*triggered, fmt.Sprintf("error_rate_%.1f%%", metrics.ErrorRate))
		exceeded = true
	}
	// Note: ResponseTimeMs and ActiveConnections would need to be added to SystemMetrics
	// if we want to use them for threshold checking

	return exceeded
}

// changeLevel changes the current degradation level
func (gd *GracefulDegradation) changeLevel(newLevel DegradationLevel, reason string, triggered []string) {
	oldLevel := gd.currentLevel
	gd.currentLevel = newLevel
	gd.levelChangedAt = time.Now()
	gd.pendingUpgrade = nil

	// Update feature states
	gd.updateFeatureStates()

	// Record the change
	event := LevelChangeEvent{
		FromLevel: oldLevel,
		ToLevel:   newLevel,
		Timestamp: gd.levelChangedAt,
		Reason:    reason,
		Triggered: triggered,
	}
	gd.levelChangeHistory = append(gd.levelChangeHistory, event)

	// Keep history manageable
	if len(gd.levelChangeHistory) > 100 {
		gd.levelChangeHistory = gd.levelChangeHistory[len(gd.levelChangeHistory)-100:]
	}

	// Log the change
	gd.logger.Info("degradation level changed",
		"from_level", oldLevel.String(),
		"to_level", newLevel.String(),
		"reason", reason,
		"triggered", triggered)

	// Calculate and log resource savings
	savings := gd.calculateResourceSavings()
	gd.logger.Info("estimated resource savings",
		"percentage", fmt.Sprintf("%.1f%%", savings*100))
}

// updateFeatureStates updates which features are enabled based on current level
func (gd *GracefulDegradation) updateFeatureStates() {
	for featureName, featureConfig := range gd.config.Features {
		gd.enabledFeatures[featureName] = gd.currentLevel <= featureConfig.EnabledAtLevel
	}
}

// calculateResourceSavings estimates the resource savings from disabled features
func (gd *GracefulDegradation) calculateResourceSavings() float64 {
	var totalWeight float64
	var disabledWeight float64

	for featureName, featureConfig := range gd.config.Features {
		totalWeight += featureConfig.ResourceWeight
		if !gd.enabledFeatures[featureName] {
			disabledWeight += featureConfig.ResourceWeight
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return disabledWeight / totalWeight
}

// GetStats returns statistics about degradation
func (gd *GracefulDegradation) GetStats() DegradationStats {
	gd.mu.RLock()
	defer gd.mu.RUnlock()

	var enabledFeatures []FeatureName
	var disabledFeatures []FeatureName
	featureStates := make(map[FeatureName]bool)

	for featureName, enabled := range gd.enabledFeatures {
		featureStates[featureName] = enabled
		if enabled {
			enabledFeatures = append(enabledFeatures, featureName)
		} else {
			disabledFeatures = append(disabledFeatures, featureName)
		}
	}

	// Create a copy of the history to avoid data races
	historyCopy := make([]LevelChangeEvent, len(gd.levelChangeHistory))
	copy(historyCopy, gd.levelChangeHistory)

	return DegradationStats{
		CurrentLevel:       gd.currentLevel,
		LevelChangedAt:     gd.levelChangedAt,
		TimeInCurrentLevel: time.Since(gd.levelChangedAt),
		EnabledFeatures:    enabledFeatures,
		DisabledFeatures:   disabledFeatures,
		ResourceSavings:    gd.calculateResourceSavings(),
		LevelChangeHistory: historyCopy,
		FeatureStates:      featureStates,
	}
}

// Reset resets the degradation manager to normal level
func (gd *GracefulDegradation) Reset() {
	gd.mu.Lock()
	defer gd.mu.Unlock()

	gd.currentLevel = LevelNormal
	gd.levelChangedAt = time.Now()
	gd.levelChangeHistory = nil
	gd.lastStableTime = time.Now()
	gd.pendingUpgrade = nil
	gd.updateFeatureStates()

	gd.logger.Info("graceful degradation reset to normal level")
}

// Stop stops the degradation manager and cleans up resources
func (gd *GracefulDegradation) Stop() {
	gd.cancel()
	gd.wg.Wait()
	gd.logger.Info("graceful degradation stopped")
}