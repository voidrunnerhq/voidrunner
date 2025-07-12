package executor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/voidrunnerhq/voidrunner/internal/models"
)

// SecurityManager handles security configuration and validation
type SecurityManager struct {
	config *Config
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config *Config) *SecurityManager {
	return &SecurityManager{
		config: config,
	}
}

// BuildSecurityConfig creates a security configuration for a given task
func (sm *SecurityManager) BuildSecurityConfig(task *models.Task) (*SecurityConfig, error) {
	if task == nil {
		return nil, NewSecurityError("build_security_config", "task is nil", nil)
	}

	// Validate script content for security issues
	if err := sm.ValidateScriptContent(task.ScriptContent, task.ScriptType); err != nil {
		return nil, fmt.Errorf("script security validation failed: %w", err)
	}

	// Build base security configuration
	securityConfig := &SecurityConfig{
		User:                sm.config.Security.ExecutionUser,
		NoNewPrivileges:     true,
		ReadOnlyRootfs:      true,
		NetworkDisabled:     true,
		DropAllCapabilities: true,
		TmpfsMounts: map[string]string{
			"/tmp":       "rw,noexec,nosuid,size=100m",
			"/var/tmp":   "rw,noexec,nosuid,size=10m",
			"/workspace": "rw,noexec,nosuid,size=50m",
		},
	}

	// Build security options
	securityOpts := []string{
		"no-new-privileges",
	}

	// Add seccomp profile if enabled
	if sm.config.Security.EnableSeccomp {
		if sm.config.Security.SeccompProfilePath != "" {
			securityOpts = append(securityOpts, "seccomp="+sm.config.Security.SeccompProfilePath)
		} else {
			// Use default seccomp profile
			securityOpts = append(securityOpts, "seccomp=unconfined")
		}
	}

	// Add AppArmor profile if enabled
	if sm.config.Security.EnableAppArmor && sm.config.Security.AppArmorProfile != "" {
		securityOpts = append(securityOpts, "apparmor="+sm.config.Security.AppArmorProfile)
	}

	securityConfig.SecurityOpts = securityOpts

	return securityConfig, nil
}

// ValidateScriptContent performs security validation on script content
func (sm *SecurityManager) ValidateScriptContent(content string, scriptType models.ScriptType) error {
	if content == "" {
		return NewSecurityError("validate_script", "script content is empty", nil)
	}

	// Convert to lowercase for case-insensitive checks
	lowerContent := strings.ToLower(content)

	// Check for script-specific dangerous patterns first
	switch scriptType {
	case models.ScriptTypePython:
		if err := sm.validatePythonScript(lowerContent); err != nil {
			return err
		}
	case models.ScriptTypeBash:
		if err := sm.validateBashScript(lowerContent); err != nil {
			return err
		}
	case models.ScriptTypeJavaScript:
		if err := sm.validateJavaScriptScript(lowerContent); err != nil {
			return err
		}
	}

	// Check for general dangerous patterns that are not script-specific
	dangerousPatterns := []string{
		// File system operations
		"rm -rf",
		"rm -r",
		"rmdir",
		"deltree",
		"format",
		"fdisk",
		"mkfs",
		"dd if=",

		// Network operations
		"wget",
		"curl",

		// Docker/container escape attempts
		"docker",
		"kubectl",
		"podman",
		"containerd",
		"runc",
		"/var/run/docker.sock",

		// Crypto mining (common patterns)
		"xmrig",
		"cpuminer",
		"ccminer",
		"minerd",
		"stratum",

		// Privilege escalation
		"setuid",
		"setgid",
		"ptrace",
		"chroot",
	}

	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerContent, pattern) {
			return NewSecurityError("validate_script",
				fmt.Sprintf("potentially dangerous pattern detected: %s", pattern), nil)
		}
	}

	return nil
}

// validatePythonScript performs Python-specific security validation
func (sm *SecurityManager) validatePythonScript(content string) error {
	// Define safe imports that are allowed
	safeImports := map[string]bool{
		"import math":             true,
		"import json":             true,
		"import datetime":         true,
		"import random":           true,
		"import time":             true,
		"import re":               true,
		"import collections":      true,
		"import itertools":        true,
		"import functools":        true,
		"import decimal":          true,
		"import fractions":        true,
		"import statistics":       true,
		"import string":           true,
		"import textwrap":         true,
		"import unicodedata":      true,
		"import base64":           true,
		"import binascii":         true,
		"import hashlib":          true,
		"import hmac":             true,
		"import uuid":             true,
		"from math import":        true,
		"from json import":        true,
		"from datetime import":    true,
		"from random import":      true,
		"from time import":        true,
		"from re import":          true,
		"from collections import": true,
		"from itertools import":   true,
		"from functools import":   true,
		"from decimal import":     true,
		"from fractions import":   true,
		"from statistics import":  true,
		"from string import":      true,
		"from textwrap import":    true,
		"from unicodedata import": true,
		"from base64 import":      true,
		"from binascii import":    true,
		"from hashlib import":     true,
		"from hmac import":        true,
		"from uuid import":        true,
	}

	// Check for dangerous patterns
	dangerousPythonPatterns := []string{
		// System and process manipulation
		"import os",
		"import subprocess",
		"import sys",
		"import shutil",
		"import tempfile",
		"import multiprocessing",
		"import threading",
		"import signal",
		"import atexit",
		"from os import",
		"from subprocess import",
		"from sys import",
		"from shutil import",
		"from tempfile import",
		"from multiprocessing import",
		"from threading import",
		"from signal import",
		"from atexit import",

		// Network and external communication
		"import socket",
		"import urllib",
		"import requests",
		"import http",
		"import smtplib",
		"import ftplib",
		"import poplib",
		"import imaplib",
		"import telnetlib",
		"from socket import",
		"from urllib import",
		"from requests import",
		"from http import",
		"from smtplib import",
		"from ftplib import",
		"from poplib import",
		"from imaplib import",
		"from telnetlib import",

		// Dynamic code execution
		"__import__",
		"eval(",
		"exec(",
		"compile(",
		"globals()",
		"locals()",
		"vars()",

		// Dangerous reflection and attribute access
		"getattr(",
		"setattr(",
		"delattr(",
		"hasattr(",

		// Dangerous file system access patterns (allow basic file operations within container)
		// Note: Container filesystem is read-only except for /tmp, so file access is limited

		// User input (can be dangerous in containers)
		"input(",
		"raw_input(",

		// Import manipulation
		"importlib",
		"__builtins__",
		"__name__",
		"__file__",
		"__doc__",
		"__package__",
		"__loader__",
		"__spec__",
		"__path__",
		"__cached__",
	}

	// Split content into lines and check each import statement
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if this line contains an import statement
		if strings.HasPrefix(line, "import ") || strings.HasPrefix(line, "from ") {
			// Check if it's a safe import
			isSafe := false
			for safeImport := range safeImports {
				if strings.HasPrefix(line, safeImport) {
					isSafe = true
					break
				}
			}

			if !isSafe {
				// Check if it's a dangerous import
				for _, pattern := range dangerousPythonPatterns {
					if strings.HasPrefix(line, pattern) {
						return NewSecurityError("validate_python_script",
							fmt.Sprintf("dangerous Python import detected: %s", pattern), nil)
					}
				}
			}
		}
	}

	// Check for dangerous patterns in the entire content
	for _, pattern := range dangerousPythonPatterns {
		// Skip import patterns as they're handled above
		if strings.HasPrefix(pattern, "import ") || strings.HasPrefix(pattern, "from ") {
			continue
		}

		if strings.Contains(content, pattern) {
			return NewSecurityError("validate_python_script",
				fmt.Sprintf("dangerous Python pattern detected: %s", pattern), nil)
		}
	}

	return nil
}

// validateBashScript performs Bash-specific security validation
func (sm *SecurityManager) validateBashScript(content string) error {
	// Define truly dangerous patterns that should be blocked
	dangerousBashPatterns := []string{
		// Network access
		"/dev/tcp/", // TCP redirection
		"/dev/udp/", // UDP redirection

		// System access and privilege escalation
		"source /",
		". /",
		"export HOME=",
		"export PATH=",
		"export LD_",
		"export DYLD_",
		"history -c",
		"history -d",
		"fc -s",

		// Dangerous background processes
		"nohup ",
		"disown ",
		"setsid ",
		"daemon ",

		// Process control
		"trap ",
		"kill -",
		"killall ",
		"pkill ",

		// File system manipulation
		"mount ",
		"umount ",
		"chroot ",
		"pivot_root ",

		// Dangerous commands
		"sudo ",
		"su ",
		"passwd ",
		"chsh ",
		"chfn ",
		"newgrp ",
		"sg ",

		// System information gathering (allow basic commands like whoami, id for educational use)
		"uname -a",
		"hostname ",
		"last ",
		"w ",
		"finger ",

		// Network commands
		"ping ",
		"traceroute ",
		"nslookup ",
		"dig ",
		"host ",
		"netstat ",
		"ss ",
		"lsof ",
		"fuser ",

		// Package management
		"apt ",
		"yum ",
		"rpm ",
		"dpkg ",
		"snap ",
		"flatpak ",
		"brew ",
		"port ",
		"emerge ",
		"pacman ",
		"zypper ",

		// Dangerous files and directories (allow /tmp/ as containers need temp space)
		"/etc/passwd",
		"/etc/shadow",
		"/etc/group",
		"/etc/sudoers",
		"/root/",
		"/home/",
		"/Users/",
		"/var/log/",
		"/var/run/",
		"/proc/",
		"/sys/",
		"/dev/",
		"/var/tmp/",

		// Dangerous redirection patterns
		"> /",
		">> /",
		"< /",
		"2> /",
		"2>> /",
		"&> /",
		"&>> /",

		// Remote access
		"ssh ",
		"scp ",
		"rsync ",
		"rcp ",
		"telnet ",
		"ftp ",
		"sftp ",
		"nc ",
		"netcat ",
		"socat ",

		// Archive and compression (can be used for data exfiltration)
		"tar ",
		"gzip ",
		"gunzip ",
		"zip ",
		"unzip ",
		"7z ",
		"rar ",
		"unrar ",

		// Dangerous environment variables
		"$HOME",
		"$PATH",
		"$USER",
		"$SHELL",
		"$PWD",
		"$OLDPWD",
		"$PS1",
		"$PS2",
		"$IFS",
		"$0",
		"$$",
		"$!",
		"$?",
	}

	// Check for dangerous patterns
	for _, pattern := range dangerousBashPatterns {
		if strings.Contains(content, pattern) {
			return NewSecurityError("validate_bash_script",
				fmt.Sprintf("dangerous Bash pattern detected: %s", pattern), nil)
		}
	}

	// Check for command substitution in dangerous contexts
	if strings.Contains(content, "$(") {
		// Allow simple command substitution for safe operations
		// Block if it contains dangerous commands within $()
		for _, pattern := range dangerousBashPatterns {
			if strings.Contains(content, "$("+pattern) {
				return NewSecurityError("validate_bash_script",
					fmt.Sprintf("dangerous command substitution detected: $(%s", pattern), nil)
			}
		}
	}

	// Check for backtick command substitution
	if strings.Contains(content, "`") {
		// Backticks are generally more dangerous, allow only very simple cases
		backtickPattern := "`[^`]*`"
		if matched, _ := regexp.MatchString(backtickPattern, content); matched {
			return NewSecurityError("validate_bash_script",
				"backtick command substitution detected (use $() instead)", nil)
		}
	}

	return nil
}

// validateJavaScriptScript performs JavaScript-specific security validation
func (sm *SecurityManager) validateJavaScriptScript(content string) error {
	// Define safe require patterns that are allowed
	safeRequirePatterns := []string{
		"require('crypto')",
		"require('util')",
		"require('path')",
		"require('url')",
		"require('querystring')",
		"require('string_decoder')",
		"require('buffer')",
		"require('events')",
		"require('stream')",
		"require('assert')",
		"require('console')",
		"require('timers')",
		"require(\"crypto\")",
		"require(\"util\")",
		"require(\"path\")",
		"require(\"url\")",
		"require(\"querystring\")",
		"require(\"string_decoder\")",
		"require(\"buffer\")",
		"require(\"events\")",
		"require(\"stream\")",
		"require(\"assert\")",
		"require(\"console\")",
		"require(\"timers\")",
	}

	// Define truly dangerous patterns that should be blocked
	dangerousJSPatterns := []string{
		// Node.js system access
		"require('fs')",
		"require('child_process')",
		"require('os')",
		"require('process')",
		"require('cluster')",
		"require('worker_threads')",
		"require('vm')",
		"require('module')",
		"require('repl')",
		"require('readline')",
		"require('tty')",
		"require(\"fs\")",
		"require(\"child_process\")",
		"require(\"os\")",
		"require(\"process\")",
		"require(\"cluster\")",
		"require(\"worker_threads\")",
		"require(\"vm\")",
		"require(\"module\")",
		"require(\"repl\")",
		"require(\"readline\")",
		"require(\"tty\")",

		// Network access
		"require('http')",
		"require('https')",
		"require('net')",
		"require('dgram')",
		"require('dns')",
		"require('tls')",
		"require(\"http\")",
		"require(\"https\")",
		"require(\"net\")",
		"require(\"dgram\")",
		"require(\"dns\")",
		"require(\"tls\")",

		// Dynamic code execution
		"eval(",
		"new Function(",
		"Function(",
		"setTimeout(",
		"setInterval(",
		"setImmediate(",

		// Process and environment access
		"process.exit(",
		"process.env",
		"process.argv",
		"process.cwd(",
		"process.chdir(",
		"process.kill(",
		"process.abort(",
		"process.platform",
		"process.version",
		"process.versions",

		// Global object access
		"global.",
		"globalThis.",
		"window.",
		"document.",
		"navigator.",
		"location.",
		"history.",
		"localStorage.",
		"sessionStorage.",

		// Dangerous constructor access
		"constructor(",
		"__proto__",
		"prototype",

		// Import statements (ES6 modules)
		"import ",
		"export ",
		"import(", // Dynamic imports

		// Dangerous eval-like functions
		"execScript(",
		"msWriteProfilerMark(",
		"webkitRequestFileSystem(",
		"webkitResolveLocalFileSystemURL(",
	}

	// Check for dangerous patterns
	for _, pattern := range dangerousJSPatterns {
		if strings.Contains(content, pattern) {
			// Check if it's a safe require pattern first
			isSafeRequire := false
			for _, safePattern := range safeRequirePatterns {
				if strings.Contains(content, safePattern) {
					isSafeRequire = true
					break
				}
			}

			if !isSafeRequire {
				return NewSecurityError("validate_javascript_script",
					fmt.Sprintf("dangerous JavaScript pattern detected: %s", pattern), nil)
			}
		}
	}

	// Check for require patterns that are not in the safe list
	requirePattern := `require\s*\(\s*['"](.*?)['"]\s*\)`
	re := regexp.MustCompile(requirePattern)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			moduleName := match[1]
			// Check if this module is in our safe list
			isSafe := false
			safeModules := []string{"crypto", "util", "path", "url", "querystring", "string_decoder", "buffer", "events", "stream", "assert", "console", "timers"}
			for _, safeModule := range safeModules {
				if moduleName == safeModule {
					isSafe = true
					break
				}
			}

			if !isSafe {
				return NewSecurityError("validate_javascript_script",
					fmt.Sprintf("unsafe require module detected: %s", moduleName), nil)
			}
		}
	}

	return nil
}

// CreateSeccompProfile creates a custom seccomp profile for the container
func (sm *SecurityManager) CreateSeccompProfile(ctx context.Context) (string, error) {
	profile := `{
    "defaultAction": "SCMP_ACT_ERRNO",
    "architectures": [
        "SCMP_ARCH_X86_64",
        "SCMP_ARCH_X86",
        "SCMP_ARCH_X32"
    ],
    "syscalls": [
        {
            "names": [
                "accept",
                "accept4",
                "access",
                "brk",
                "close",
                "dup",
                "dup2",
                "exit",
                "exit_group",
                "fstat",
                "fstat64",
                "getdents",
                "getdents64",
                "getpid",
                "getuid",
                "getgid",
                "geteuid",
                "getegid",
                "lseek",
                "lstat",
                "lstat64",
                "mmap",
                "mmap2",
                "mprotect",
                "munmap",
                "newfstatat",
                "open",
                "openat",
                "poll",
                "ppoll",
                "read",
                "readlink",
                "readlinkat",
                "rt_sigaction",
                "rt_sigprocmask",
                "rt_sigreturn",
                "select",
                "stat",
                "stat64",
                "statfs",
                "statfs64",
                "write",
                "writev"
            ],
            "action": "SCMP_ACT_ALLOW"
        }
    ]
}`

	// Determine secure directory for profile storage
	tempDir, err := sm.getSecureProfileDirectory()
	if err != nil {
		return "", NewSecurityError("create_seccomp_profile", "failed to get secure directory", err)
	}

	// Validate directory exists and has proper permissions
	if err := sm.validateSecureDirectory(tempDir); err != nil {
		return "", NewSecurityError("create_seccomp_profile", "directory security validation failed", err)
	}

	profilePath := filepath.Join(tempDir, "seccomp-profile.json")

	// Write profile with secure permissions (0600 - owner read/write only)
	err = os.WriteFile(profilePath, []byte(profile), 0600)
	if err != nil {
		return "", NewSecurityError("create_seccomp_profile", "failed to write seccomp profile", err)
	}

	// Verify file permissions after creation
	if err := sm.validateFilePermissions(profilePath, 0600); err != nil {
		// Clean up the file if permissions are wrong
		_ = os.Remove(profilePath) // Ignore cleanup errors
		return "", NewSecurityError("create_seccomp_profile", "file permission validation failed", err)
	}

	return profilePath, nil
}

// getSecureProfileDirectory determines the most secure directory for profile storage
func (sm *SecurityManager) getSecureProfileDirectory() (string, error) {
	// Preferred directories in order of security preference
	preferredDirs := []string{
		"/opt/voidrunner/profiles",
		"/opt/voidrunner",
		"/var/lib/voidrunner",
		"/tmp/voidrunner-profiles",
	}

	for _, dir := range preferredDirs {
		// Check if directory exists or can be created
		if err := os.MkdirAll(dir, 0700); err != nil {
			continue // Try next directory
		}

		// Verify we can write to the directory
		testFile := filepath.Join(dir, ".write-test")
		if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
			continue // Try next directory
		}
		_ = os.Remove(testFile) // Clean up test file, ignore errors

		return dir, nil
	}

	// Fallback to system temp directory with restricted subdirectory
	tempDir := filepath.Join(os.TempDir(), "voidrunner-profiles")
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create fallback temp directory: %w", err)
	}

	return tempDir, nil
}

// validateSecureDirectory validates that a directory has appropriate security settings
func (sm *SecurityManager) validateSecureDirectory(dirPath string) error {
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("directory does not exist: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dirPath)
	}

	// Check directory permissions (should be 0700 - owner only)
	mode := info.Mode().Perm()
	if mode != 0700 {
		return fmt.Errorf("directory has insecure permissions %o, expected 0700", mode)
	}

	return nil
}

// validateFilePermissions validates that a file has the expected permissions
func (sm *SecurityManager) validateFilePermissions(filePath string, expectedMode os.FileMode) error {
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file does not exist: %w", err)
	}

	actualMode := info.Mode().Perm()
	if actualMode != expectedMode {
		return fmt.Errorf("file has incorrect permissions %o, expected %o", actualMode, expectedMode)
	}

	return nil
}

// ValidateContainerConfig validates that a container configuration meets security requirements
func (sm *SecurityManager) ValidateContainerConfig(config *ContainerConfig) error {
	if config == nil {
		return NewSecurityError("validate_container_config", "container config is nil", nil)
	}

	// Comprehensive security configuration validation
	if err := sm.validateSecurityConfig(&config.SecurityConfig); err != nil {
		return fmt.Errorf("security config validation failed: %w", err)
	}

	// Comprehensive resource limits validation
	if err := sm.validateResourceLimits(&config.ResourceLimits); err != nil {
		return fmt.Errorf("resource limits validation failed: %w", err)
	}

	// Validate container image security
	if err := sm.CheckImageSecurity(config.Image); err != nil {
		return fmt.Errorf("image security validation failed: %w", err)
	}

	// Validate working directory security
	if err := sm.validateWorkingDirectory(config.WorkingDir); err != nil {
		return fmt.Errorf("working directory validation failed: %w", err)
	}

	// Validate environment variables
	if err := sm.validateEnvironmentVariables(config.Environment); err != nil {
		return fmt.Errorf("environment validation failed: %w", err)
	}

	// Validate timeout
	if config.Timeout <= 0 {
		return NewSecurityError("validate_container_config", "timeout must be positive", nil)
	}

	if int(config.Timeout.Seconds()) > sm.config.Security.MaxTimeoutSeconds {
		return NewSecurityError("validate_container_config",
			fmt.Sprintf("timeout exceeds maximum allowed (%d seconds)", sm.config.Security.MaxTimeoutSeconds), nil)
	}

	return nil
}

// validateSecurityConfig performs comprehensive security configuration validation
func (sm *SecurityManager) validateSecurityConfig(config *SecurityConfig) error {
	// Ensure non-root execution
	if config.User == "" {
		return NewSecurityError("validate_security_config", "user must be specified", nil)
	}

	// Parse and validate user specification
	if err := sm.validateUserSpecification(config.User); err != nil {
		return fmt.Errorf("user specification validation failed: %w", err)
	}

	// Ensure read-only root filesystem
	if !config.ReadOnlyRootfs {
		return NewSecurityError("validate_security_config", "read-only root filesystem must be enabled", nil)
	}

	// Ensure network is disabled
	if !config.NetworkDisabled {
		return NewSecurityError("validate_security_config", "network must be disabled for security", nil)
	}

	// Ensure no new privileges
	if !config.NoNewPrivileges {
		return NewSecurityError("validate_security_config", "no-new-privileges must be enabled", nil)
	}

	// Ensure all capabilities are dropped
	if !config.DropAllCapabilities {
		return NewSecurityError("validate_security_config", "all capabilities must be dropped", nil)
	}

	// Validate security options
	if err := sm.validateSecurityOptions(config.SecurityOpts); err != nil {
		return fmt.Errorf("security options validation failed: %w", err)
	}

	// Validate tmpfs mounts
	if err := sm.validateTmpfsMounts(config.TmpfsMounts); err != nil {
		return fmt.Errorf("tmpfs mounts validation failed: %w", err)
	}

	return nil
}

// validateResourceLimits performs comprehensive resource limits validation
func (sm *SecurityManager) validateResourceLimits(limits *ResourceLimits) error {
	// Memory validation
	if limits.MemoryLimitBytes <= 0 {
		return NewSecurityError("validate_resource_limits", "memory limit must be positive", nil)
	}

	if limits.MemoryLimitBytes > sm.config.Security.MaxMemoryLimitBytes {
		return NewSecurityError("validate_resource_limits",
			fmt.Sprintf("memory limit (%d bytes) exceeds maximum allowed (%d bytes)",
				limits.MemoryLimitBytes, sm.config.Security.MaxMemoryLimitBytes), nil)
	}

	// CPU validation
	if limits.CPUQuota <= 0 {
		return NewSecurityError("validate_resource_limits", "CPU quota must be positive", nil)
	}

	if limits.CPUQuota > sm.config.Security.MaxCPUQuota {
		return NewSecurityError("validate_resource_limits",
			fmt.Sprintf("CPU quota (%d) exceeds maximum allowed (%d)",
				limits.CPUQuota, sm.config.Security.MaxCPUQuota), nil)
	}

	// PID validation
	if limits.PidsLimit <= 0 {
		return NewSecurityError("validate_resource_limits", "PID limit must be positive", nil)
	}

	if limits.PidsLimit > sm.config.Security.MaxPidsLimit {
		return NewSecurityError("validate_resource_limits",
			fmt.Sprintf("PID limit (%d) exceeds maximum allowed (%d)",
				limits.PidsLimit, sm.config.Security.MaxPidsLimit), nil)
	}

	// Timeout validation
	if limits.TimeoutSeconds <= 0 {
		return NewSecurityError("validate_resource_limits", "timeout must be positive", nil)
	}

	if limits.TimeoutSeconds > sm.config.Security.MaxTimeoutSeconds {
		return NewSecurityError("validate_resource_limits",
			fmt.Sprintf("timeout (%d seconds) exceeds maximum allowed (%d seconds)",
				limits.TimeoutSeconds, sm.config.Security.MaxTimeoutSeconds), nil)
	}

	return nil
}

// validateUserSpecification validates the user specification format
func (sm *SecurityManager) validateUserSpecification(user string) error {
	// Check for root user patterns
	if user == "root" || user == "0" || user == "0:0" {
		return NewSecurityError("validate_user_specification", "root user execution is not allowed", nil)
	}

	// Validate UID:GID format
	parts := strings.Split(user, ":")
	if len(parts) != 2 {
		return NewSecurityError("validate_user_specification", "user must be in UID:GID format", nil)
	}

	// Validate UID
	uid := parts[0]
	if uid == "0" {
		return NewSecurityError("validate_user_specification", "UID 0 (root) is not allowed", nil)
	}

	// Validate GID
	gid := parts[1]
	if gid == "0" {
		return NewSecurityError("validate_user_specification", "GID 0 (root) is not allowed", nil)
	}

	return nil
}

// validateSecurityOptions validates security options
func (sm *SecurityManager) validateSecurityOptions(opts []string) error {
	requiredOpts := map[string]bool{
		"no-new-privileges": false,
	}

	for _, opt := range opts {
		if strings.HasPrefix(opt, "no-new-privileges") {
			requiredOpts["no-new-privileges"] = true
		}

		// Validate seccomp options
		if strings.HasPrefix(opt, "seccomp=") {
			seccompProfile := strings.TrimPrefix(opt, "seccomp=")
			if seccompProfile != "unconfined" && seccompProfile != "" {
				// Validate seccomp profile path
				if err := sm.validateSeccompProfilePath(seccompProfile); err != nil {
					return fmt.Errorf("seccomp profile validation failed: %w", err)
				}
			}
		}

		// Validate AppArmor options
		if strings.HasPrefix(opt, "apparmor=") {
			profile := strings.TrimPrefix(opt, "apparmor=")
			if profile == "unconfined" {
				return NewSecurityError("validate_security_options", "AppArmor unconfined profile is not allowed", nil)
			}
		}
	}

	// Check that all required options are present
	for opt, present := range requiredOpts {
		if !present {
			return NewSecurityError("validate_security_options",
				fmt.Sprintf("required security option '%s' is missing", opt), nil)
		}
	}

	return nil
}

// validateTmpfsMounts validates tmpfs mount configurations
func (sm *SecurityManager) validateTmpfsMounts(mounts map[string]string) error {
	allowedPaths := map[string]bool{
		"/tmp":       true,
		"/var/tmp":   true,
		"/workspace": true,
		"/run":       true,
	}

	for path, options := range mounts {
		// Validate mount path
		if !allowedPaths[path] {
			return NewSecurityError("validate_tmpfs_mounts",
				fmt.Sprintf("tmpfs mount path '%s' is not allowed", path), nil)
		}

		// Validate mount options
		if err := sm.validateTmpfsOptions(options); err != nil {
			return fmt.Errorf("tmpfs options validation failed for path '%s': %w", path, err)
		}
	}

	return nil
}

// validateTmpfsOptions validates tmpfs mount options
func (sm *SecurityManager) validateTmpfsOptions(options string) error {
	requiredOptions := map[string]bool{
		"noexec": false,
		"nosuid": false,
	}

	opts := strings.Split(options, ",")
	for _, opt := range opts {
		opt = strings.TrimSpace(opt)

		if opt == "noexec" {
			requiredOptions["noexec"] = true
		}
		if opt == "nosuid" {
			requiredOptions["nosuid"] = true
		}

		// Check for dangerous options
		if opt == "exec" {
			return NewSecurityError("validate_tmpfs_options", "exec option is not allowed in tmpfs mounts", nil)
		}
		if opt == "suid" {
			return NewSecurityError("validate_tmpfs_options", "suid option is not allowed in tmpfs mounts", nil)
		}
	}

	// Check that all required options are present
	for opt, present := range requiredOptions {
		if !present {
			return NewSecurityError("validate_tmpfs_options",
				fmt.Sprintf("required tmpfs option '%s' is missing", opt), nil)
		}
	}

	return nil
}

// validateWorkingDirectory validates the container working directory
func (sm *SecurityManager) validateWorkingDirectory(workingDir string) error {
	if workingDir == "" {
		return NewSecurityError("validate_working_directory", "working directory must be specified", nil)
	}

	// Ensure it's an absolute path
	if !strings.HasPrefix(workingDir, "/") {
		return NewSecurityError("validate_working_directory", "working directory must be an absolute path", nil)
	}

	// Prevent access to sensitive directories
	dangerousPaths := []string{
		"/etc", "/root", "/home", "/var/log", "/var/run", "/proc", "/sys", "/dev",
		"/bin", "/sbin", "/usr/bin", "/usr/sbin", "/opt/voidrunner",
	}

	for _, dangerousPath := range dangerousPaths {
		if strings.HasPrefix(workingDir, dangerousPath) {
			return NewSecurityError("validate_working_directory",
				fmt.Sprintf("working directory '%s' is in restricted path '%s'", workingDir, dangerousPath), nil)
		}
	}

	return nil
}

// validateEnvironmentVariables validates environment variables for security
func (sm *SecurityManager) validateEnvironmentVariables(env []string) error {
	for _, envVar := range env {
		if envVar == "" {
			continue
		}

		// Check for dangerous environment variables
		upperEnvVar := strings.ToUpper(envVar)

		dangerousPatterns := []string{
			"LD_PRELOAD=", "LD_LIBRARY_PATH=", "DOCKER_HOST=", "KUBERNETES_",
			"SECRET=", "PASSWORD=", "TOKEN=", "KEY=", "CREDENTIAL=",
		}

		for _, pattern := range dangerousPatterns {
			if strings.HasPrefix(upperEnvVar, pattern) {
				return NewSecurityError("validate_environment_variables",
					fmt.Sprintf("dangerous environment variable pattern detected: %s", pattern), nil)
			}
		}
	}

	return nil
}

// validateSeccompProfilePath validates a seccomp profile path
func (sm *SecurityManager) validateSeccompProfilePath(profilePath string) error {
	if profilePath == "" {
		return NewSecurityError("validate_seccomp_profile", "seccomp profile path is empty", nil)
	}

	// Ensure it's an absolute path
	if !strings.HasPrefix(profilePath, "/") {
		return NewSecurityError("validate_seccomp_profile", "seccomp profile path must be absolute", nil)
	}

	// Validate file exists and has correct permissions
	if err := sm.validateFilePermissions(profilePath, 0600); err != nil {
		return fmt.Errorf("seccomp profile file validation failed: %w", err)
	}

	return nil
}

// GenerateContainerName generates a secure, unique container name
func (sm *SecurityManager) GenerateContainerName(taskID string) string {
	// Use a predictable but unique naming pattern
	return fmt.Sprintf("voidrunner-task-%s", taskID)
}

// CheckImageSecurity validates that a container image is safe to use
func (sm *SecurityManager) CheckImageSecurity(image string) error {
	if image == "" {
		return NewSecurityError("check_image_security", "image name is empty", nil)
	}

	// Whitelist of allowed base images
	allowedImages := map[string]bool{
		"python:3.11-alpine": true,
		"python:3.10-alpine": true,
		"python:3.9-alpine":  true,
		"alpine:latest":      true,
		"alpine:3.18":        true,
		"alpine:3.17":        true,
		"node:18-alpine":     true,
		"node:16-alpine":     true,
		"golang:1.21-alpine": true,
		"golang:1.20-alpine": true,
	}

	if !allowedImages[image] {
		return NewSecurityError("check_image_security",
			fmt.Sprintf("image %s is not in the allowed list", image), nil)
	}

	return nil
}

// SanitizeEnvironment sanitizes environment variables for security
func (sm *SecurityManager) SanitizeEnvironment(env []string) []string {
	var sanitized []string

	// Allowed environment variable prefixes
	allowedPrefixes := []string{
		"PATH=",
		"HOME=",
		"USER=",
		"LANG=",
		"LC_",
		"TZ=",
		"PYTHONPATH=",
		"PYTHONIOENCODING=",
		"NODE_ENV=",
		"GOPATH=",
		"GOOS=",
		"GOARCH=",
	}

	// Dangerous environment variables to exclude
	dangerousVars := []string{
		"LD_PRELOAD",
		"LD_LIBRARY_PATH",
		"DOCKER_HOST",
		"KUBERNETES_",
		"AWS_",
		"AZURE_",
		"GCP_",
		"SECRET",
		"PASSWORD",
		"TOKEN",
		"KEY",
		"CREDENTIAL",
	}

	for _, envVar := range env {
		allowed := false

		// Check if it starts with an allowed prefix
		for _, prefix := range allowedPrefixes {
			if strings.HasPrefix(envVar, prefix) {
				allowed = true
				break
			}
		}

		if !allowed {
			continue
		}

		// Check if it contains dangerous patterns
		upperEnvVar := strings.ToUpper(envVar)
		dangerous := false
		for _, dangerousVar := range dangerousVars {
			if strings.Contains(upperEnvVar, dangerousVar) {
				dangerous = true
				break
			}
		}

		if !dangerous {
			sanitized = append(sanitized, envVar)
		}
	}

	// Add required safe environment variables
	safeDefaults := []string{
		"PATH=/usr/local/bin:/usr/bin:/bin",
		"HOME=/tmp",
		"USER=executor",
		"PYTHONIOENCODING=utf-8",
	}

	sanitized = append(sanitized, safeDefaults...)

	return sanitized
}
