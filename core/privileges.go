package core

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strings"
)

// IsAdmin checks if the current process is running with administrative privileges
func IsAdmin() bool {
	if IsWindows() {
		return isWindowsAdmin()
	}
	return isUnixAdmin()
}

// isWindowsAdmin checks if running as administrator on Windows
func isWindowsAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

// isUnixAdmin checks if running as root on Unix-like systems (macOS/Linux)
func isUnixAdmin() bool {
	// Check if running as root (UID 0)
	return os.Geteuid() == 0
}

// CanEscalate checks if privilege escalation is possible
func CanEscalate() bool {
	if IsWindows() {
		return canWindowsEscalate()
	}
	return canUnixEscalate()
}

// canWindowsEscalate checks if UAC elevation is available on Windows
func canWindowsEscalate() bool {
	// Check if we can access UAC - this is a simple check
	// In a real implementation, you might want to check for UAC settings
	return !isWindowsAdmin()
}

// canUnixEscalate checks if sudo is available on Unix-like systems
func canUnixEscalate() bool {
	if isUnixAdmin() {
		return false // Already admin
	}

	// Check if sudo is available - that's sufficient for escalation capability
	_, err := exec.LookPath("sudo")
	return err == nil
}

// EscalatePrivileges attempts to restart the program with elevated privileges
func EscalatePrivileges() error {
	if IsAdmin() {
		return fmt.Errorf("already running with administrative privileges")
	}

	if IsWindows() {
		return escalateWindows()
	}
	return escalateUnix()
}

// escalateWindows restarts the program with UAC elevation on Windows
func escalateWindows() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Use PowerShell to trigger UAC
	args := []string{
		"-Command",
		fmt.Sprintf("Start-Process -FilePath '%s' -ArgumentList '%s' -Verb RunAs",
			executable, strings.Join(os.Args[1:], "' '")),
	}

	cmd := exec.Command("powershell.exe", args...)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to escalate privileges: %v", err)
	}

	// Exit current process after launching elevated version
	os.Exit(0)
	return nil
}

// escalateUnix restarts the program with sudo on Unix-like systems
func escalateUnix() error {
	if !CanEscalate() {
		return fmt.Errorf("cannot escalate privileges: sudo not available or not configured")
	}

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Prepare sudo command
	args := append([]string{executable}, os.Args[1:]...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to escalate privileges with sudo: %v", err)
	}

	// Wait for the elevated process to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("elevated process failed: %v", err)
	}

	// Exit current process after elevated version completes
	os.Exit(0)
	return nil
}

// GetCurrentUser returns information about the current user
func GetCurrentUser() (*user.User, error) {
	return user.Current()
}

// GetUserPrivilegeInfo returns a string describing current privilege level
func GetUserPrivilegeInfo() string {
	currentUser, err := GetCurrentUser()
	if err != nil {
		return "Unable to determine user information"
	}

	var privilegeLevel string
	if IsAdmin() {
		if IsWindows() {
			privilegeLevel = "Administrator"
		} else {
			privilegeLevel = "Root"
		}
	} else {
		privilegeLevel = "Standard User"
	}

	var escalationStatus string
	if CanEscalate() {
		escalationStatus = "Can escalate"
	} else {
		escalationStatus = "Cannot escalate"
	}

	return fmt.Sprintf("User: %s, Privilege Level: %s, %s",
		currentUser.Username, privilegeLevel, escalationStatus)
}
