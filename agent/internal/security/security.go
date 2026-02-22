package security

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Validator enforces security policies for command execution
// and file transfer operations.
type Validator struct {
	mode         string
	whitelist    []string
	blacklist    []string
	uploadDirs   []string
	downloadDirs []string
}

// New creates a new Validator with the given security configuration.
func New(mode string, whitelist, blacklist, uploadDirs, downloadDirs []string) *Validator {
	// Normalize upload and download dirs to absolute, clean paths
	normalizedUpload := make([]string, 0, len(uploadDirs))
	for _, dir := range uploadDirs {
		abs, err := filepath.Abs(filepath.Clean(dir))
		if err == nil {
			normalizedUpload = append(normalizedUpload, abs)
		}
	}

	normalizedDownload := make([]string, 0, len(downloadDirs))
	for _, dir := range downloadDirs {
		abs, err := filepath.Abs(filepath.Clean(dir))
		if err == nil {
			normalizedDownload = append(normalizedDownload, abs)
		}
	}

	return &Validator{
		mode:         mode,
		whitelist:    whitelist,
		blacklist:    blacklist,
		uploadDirs:   normalizedUpload,
		downloadDirs: normalizedDownload,
	}
}

// ValidateCommand checks whether a command is allowed based on
// the configured security mode (whitelist or blacklist).
func (v *Validator) ValidateCommand(command string) error {
	if command == "" {
		return fmt.Errorf("empty command")
	}

	switch v.mode {
	case "whitelist":
		return v.validateWhitelist(command)
	case "blacklist":
		return v.validateBlacklist(command)
	default:
		return fmt.Errorf("unknown security mode: %s", v.mode)
	}
}

// validateWhitelist extracts the first word of the command and checks
// if it is in the whitelist.
func (v *Validator) validateWhitelist(command string) error {
	cmd := extractCommand(command)
	for _, allowed := range v.whitelist {
		if cmd == allowed {
			return nil
		}
	}
	return fmt.Errorf("command %q is not in the whitelist", cmd)
}

// validateBlacklist checks if any blacklisted pattern appears as a
// substring of the command.
func (v *Validator) validateBlacklist(command string) error {
	for _, blocked := range v.blacklist {
		if strings.Contains(command, blocked) {
			return fmt.Errorf("command contains blocked pattern %q", blocked)
		}
	}
	return nil
}

// extractCommand returns the first word (token) of a command string.
func extractCommand(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// ValidateUploadPath verifies that destPath is under one of the
// allowed upload directories.
func (v *Validator) ValidateUploadPath(destPath string) error {
	return v.validatePath(destPath, v.uploadDirs, "upload")
}

// ValidateDownloadPath verifies that srcPath is under one of the
// allowed download directories.
func (v *Validator) ValidateDownloadPath(srcPath string) error {
	return v.validatePath(srcPath, v.downloadDirs, "download")
}

// validatePath checks that the given path, after cleaning and
// resolving to an absolute path, falls under one of the allowed
// directories. This prevents path traversal attacks.
func (v *Validator) validatePath(path string, allowedDirs []string, operation string) error {
	if len(allowedDirs) == 0 {
		return fmt.Errorf("no %s directories configured", operation)
	}

	// Clean and resolve to absolute path
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("invalid path %q: %w", path, err)
	}

	for _, dir := range allowedDirs {
		// Check if the clean path starts with the allowed directory
		if strings.HasPrefix(cleanPath, dir+string(filepath.Separator)) || cleanPath == dir {
			return nil
		}
	}

	return fmt.Errorf("path %q is not under any allowed %s directory", path, operation)
}
