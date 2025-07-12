package executor

import (
	"time"

	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// Config represents the configuration for the executor
type Config struct {
	// Docker daemon endpoint
	DockerEndpoint string

	// Default resource limits
	DefaultResourceLimits ResourceLimits

	// Default execution timeout
	DefaultTimeoutSeconds int

	// Container image configurations
	Images ImageConfig

	// Security settings
	Security SecuritySettings
}

// ImageConfig defines container images for different script types
type ImageConfig struct {
	// Python execution image
	Python string

	// Bash execution image
	Bash string

	// JavaScript execution image (for future use)
	JavaScript string

	// Go execution image (for future use)
	Go string
}

// SecuritySettings defines security configuration
type SecuritySettings struct {
	// Enable seccomp filtering
	EnableSeccomp bool

	// Path to seccomp profile
	SeccompProfilePath string

	// Enable AppArmor
	EnableAppArmor bool

	// AppArmor profile name
	AppArmorProfile string

	// Execution user (UID:GID)
	ExecutionUser string

	// Allowed syscalls (for custom seccomp profiles)
	AllowedSyscalls []string

	// Maximum allowed memory limit in bytes (safety cap)
	MaxMemoryLimitBytes int64

	// Maximum allowed CPU quota (safety cap)
	MaxCPUQuota int64

	// Maximum allowed PID limit (safety cap)
	MaxPidsLimit int64

	// Maximum allowed timeout in seconds (safety cap)
	MaxTimeoutSeconds int
}

// NewDefaultConfig returns a default configuration for the executor
func NewDefaultConfig() *Config {
	return &Config{
		DockerEndpoint: "unix:///var/run/docker.sock",
		DefaultResourceLimits: ResourceLimits{
			MemoryLimitBytes: 128 * 1024 * 1024, // 128MB
			CPUQuota:         50000,             // 0.5 CPU cores
			PidsLimit:        128,               // Max 128 processes
			TimeoutSeconds:   300,               // 5 minutes
		},
		DefaultTimeoutSeconds: 300,
		Images: ImageConfig{
			Python:     "python:3.11-alpine",
			Bash:       "alpine:latest",
			JavaScript: "node:18-alpine",
			Go:         "golang:1.21-alpine",
		},
		Security: SecuritySettings{
			EnableSeccomp:      true,
			SeccompProfilePath: "/opt/voidrunner/seccomp-profile.json",
			EnableAppArmor:     false,
			AppArmorProfile:    "voidrunner-executor",
			ExecutionUser:      "1000:1000",
			// Security caps to prevent resource escalation beyond safe limits
			MaxMemoryLimitBytes: 1024 * 1024 * 1024, // 1GB maximum
			MaxCPUQuota:         200000,             // 2.0 CPU cores maximum
			MaxPidsLimit:        1000,               // 1000 processes maximum
			MaxTimeoutSeconds:   3600,               // 1 hour maximum
			AllowedSyscalls: []string{
				"read", "write", "open", "close", "stat", "fstat", "lstat",
				"poll", "lseek", "mmap", "mprotect", "munmap", "brk",
				"access", "pipe", "select", "dup", "dup2", "getpid",
				"socket", "connect", "accept", "bind", "listen", "getsockname",
				"getpeername", "socketpair", "setsockopt", "getsockopt",
				"wait4", "kill", "uname", "fcntl", "flock", "fsync",
				"getcwd", "chdir", "rename", "mkdir", "rmdir", "creat",
				"unlink", "readlink", "chmod", "fchmod", "chown", "fchown",
				"umask", "gettimeofday", "getrlimit", "getrusage", "sysinfo",
				"times", "getuid", "getgid", "setuid", "setgid", "geteuid",
				"getegid", "getppid", "getpgrp", "setsid", "setreuid",
				"setregid", "getgroups", "setgroups", "setresuid", "getresuid",
				"setresgid", "getresgid", "getpgid", "setpgid", "getsid",
				"sethostname", "setrlimit", "getrlimit", "getrusage", "umask",
				"prctl", "getcpu", "exit", "exit_group",
			},
		},
	}
}

// GetImageForScriptType returns the appropriate container image for the given script type
func (c *Config) GetImageForScriptType(scriptType models.ScriptType) string {
	switch scriptType {
	case models.ScriptTypePython:
		return c.Images.Python
	case models.ScriptTypeBash:
		return c.Images.Bash
	case models.ScriptTypeJavaScript:
		return c.Images.JavaScript
	case models.ScriptTypeGo:
		return c.Images.Go
	default:
		return c.Images.Python // Default to Python
	}
}

// GetResourceLimitsForTask returns resource limits for a specific task
func (c *Config) GetResourceLimitsForTask(task *models.Task) ResourceLimits {
	limits := c.DefaultResourceLimits

	// Use task-specific timeout if specified
	if task.TimeoutSeconds > 0 {
		limits.TimeoutSeconds = task.TimeoutSeconds
	}

	// Apply priority-based resource scaling
	switch task.Priority {
	case 0, 1, 2: // Low priority
		limits.MemoryLimitBytes = c.DefaultResourceLimits.MemoryLimitBytes / 2
		limits.CPUQuota = c.DefaultResourceLimits.CPUQuota / 2
	case 3, 4, 5: // Normal priority
		// Use defaults
	case 6, 7, 8: // High priority
		limits.MemoryLimitBytes = c.DefaultResourceLimits.MemoryLimitBytes * 2
		limits.CPUQuota = c.DefaultResourceLimits.CPUQuota * 2
	case 9, 10: // Critical priority
		limits.MemoryLimitBytes = c.DefaultResourceLimits.MemoryLimitBytes * 4
		limits.CPUQuota = c.DefaultResourceLimits.CPUQuota * 2
	}

	// Apply security caps to prevent resource escalation beyond safe limits
	limits = c.applySecurityCaps(limits)

	return limits
}

// applySecurityCaps enforces maximum resource limits for security
func (c *Config) applySecurityCaps(limits ResourceLimits) ResourceLimits {
	// Cap memory limit
	if limits.MemoryLimitBytes > c.Security.MaxMemoryLimitBytes {
		limits.MemoryLimitBytes = c.Security.MaxMemoryLimitBytes
	}

	// Cap CPU quota
	if limits.CPUQuota > c.Security.MaxCPUQuota {
		limits.CPUQuota = c.Security.MaxCPUQuota
	}

	// Cap PID limit
	if limits.PidsLimit > c.Security.MaxPidsLimit {
		limits.PidsLimit = c.Security.MaxPidsLimit
	}

	// Cap timeout
	if limits.TimeoutSeconds > c.Security.MaxTimeoutSeconds {
		limits.TimeoutSeconds = c.Security.MaxTimeoutSeconds
	}

	return limits
}

// GetSecurityConfigForTask returns security configuration for a specific task
func (c *Config) GetSecurityConfigForTask(task *models.Task) SecurityConfig {
	securityOpts := []string{
		"no-new-privileges",
	}

	if c.Security.EnableSeccomp {
		securityOpts = append(securityOpts, "seccomp="+c.Security.SeccompProfilePath)
	}

	if c.Security.EnableAppArmor {
		securityOpts = append(securityOpts, "apparmor="+c.Security.AppArmorProfile)
	}

	return SecurityConfig{
		User:            c.Security.ExecutionUser,
		NoNewPrivileges: true,
		ReadOnlyRootfs:  true,
		NetworkDisabled: true,
		SecurityOpts:    securityOpts,
		TmpfsMounts: map[string]string{
			"/tmp":     "rw,noexec,nosuid,size=100m",
			"/var/tmp": "rw,noexec,nosuid,size=10m",
		},
		DropAllCapabilities: true,
	}
}

// GetTimeoutForTask returns the execution timeout for a specific task
func (c *Config) GetTimeoutForTask(task *models.Task) time.Duration {
	if task.TimeoutSeconds > 0 {
		return time.Duration(task.TimeoutSeconds) * time.Second
	}
	return time.Duration(c.DefaultTimeoutSeconds) * time.Second
}

// Validate validates the executor configuration
func (c *Config) Validate() error {
	if c.DefaultResourceLimits.MemoryLimitBytes <= 0 {
		return ErrInvalidConfig("memory limit must be positive")
	}

	if c.DefaultResourceLimits.CPUQuota <= 0 {
		return ErrInvalidConfig("CPU quota must be positive")
	}

	if c.DefaultResourceLimits.PidsLimit <= 0 {
		return ErrInvalidConfig("PID limit must be positive")
	}

	if c.DefaultTimeoutSeconds <= 0 {
		return ErrInvalidConfig("timeout must be positive")
	}

	if c.Images.Python == "" {
		return ErrInvalidConfig("Python image must be specified")
	}

	if c.Images.Bash == "" {
		return ErrInvalidConfig("Bash image must be specified")
	}

	// Validate security limits
	if c.Security.MaxMemoryLimitBytes <= 0 {
		return ErrInvalidConfig("maximum memory limit must be positive")
	}

	if c.Security.MaxCPUQuota <= 0 {
		return ErrInvalidConfig("maximum CPU quota must be positive")
	}

	if c.Security.MaxPidsLimit <= 0 {
		return ErrInvalidConfig("maximum PID limit must be positive")
	}

	if c.Security.MaxTimeoutSeconds <= 0 {
		return ErrInvalidConfig("maximum timeout must be positive")
	}

	// Validate that default limits don't exceed security caps
	if c.DefaultResourceLimits.MemoryLimitBytes > c.Security.MaxMemoryLimitBytes {
		return ErrInvalidConfig("default memory limit exceeds security maximum")
	}

	if c.DefaultResourceLimits.CPUQuota > c.Security.MaxCPUQuota {
		return ErrInvalidConfig("default CPU quota exceeds security maximum")
	}

	if c.DefaultResourceLimits.PidsLimit > c.Security.MaxPidsLimit {
		return ErrInvalidConfig("default PID limit exceeds security maximum")
	}

	if c.DefaultTimeoutSeconds > c.Security.MaxTimeoutSeconds {
		return ErrInvalidConfig("default timeout exceeds security maximum")
	}

	return nil
}
