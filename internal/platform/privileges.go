package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/log"
)

// IsAdmin checks if the current process is running with administrative privileges
func IsAdmin() bool {
	switch runtime.GOOS {
	case "windows":
		return isAdminWindows()
	case "darwin", "linux":
		return isAdminUnix()
	default:
		log.Warnf("Unsupported platform: %s", runtime.GOOS)
		return false
	}
}

// CanEscalate checks if we can potentially escalate privileges
func CanEscalate() bool {
	switch runtime.GOOS {
	case "windows":
		return canEscalateWindows()
	case "darwin", "linux":
		return canEscalateUnix()
	default:
		return false
	}
}

// EscalatePrivileges attempts to escalate privileges
func EscalatePrivileges() error {
	switch runtime.GOOS {
	case "windows":
		return escalatePrivilegesWindows()
	case "darwin", "linux":
		return escalatePrivilegesUnix()
	default:
		return fmt.Errorf("privilege escalation not supported on %s", runtime.GOOS)
	}
}

// GetUserPrivilegeInfo returns information about current user privileges
func GetUserPrivilegeInfo() string {
	switch runtime.GOOS {
	case "windows":
		return getUserPrivilegeInfoWindows()
	case "darwin", "linux":
		return getUserPrivilegeInfoUnix()
	default:
		return fmt.Sprintf("Unknown platform: %s", runtime.GOOS)
	}
}

// Windows-specific implementations
func isAdminWindows() bool {
	cmd := exec.Command("net", "session")
	err := cmd.Run()
	return err == nil
}

func canEscalateWindows() bool {
	// On Windows, we can try to use UAC
	return true
}

func escalatePrivilegesWindows() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Use PowerShell to run as admin
	cmd := exec.Command("powershell", "-Command",
		fmt.Sprintf("Start-Process '%s' -Verb RunAs -ArgumentList '%s'",
			exe, strings.Join(os.Args[1:], "' '")))

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to escalate privileges: %v", err)
	}

	// Exit current process since we're starting a new elevated one
	os.Exit(0)
	return nil
}

func getUserPrivilegeInfoWindows() string {
	cmd := exec.Command("whoami", "/priv")
	output, err := cmd.Output()
	if err != nil {
		return "Unable to get privilege information"
	}
	return string(output)
}

// Unix (macOS/Linux) implementations
func isAdminUnix() bool {
	return os.Geteuid() == 0
}

func canEscalateUnix() bool {
	// Check if sudo is available
	cmd := exec.Command("which", "sudo")
	err := cmd.Run()
	return err == nil
}

func escalatePrivilegesUnix() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Use sudo to re-run with elevated privileges
	args := append([]string{exe}, os.Args[1:]...)
	cmd := exec.Command("sudo", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to escalate privileges: %v", err)
	}

	// Wait for the elevated process to complete
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("elevated process failed: %v", err)
	}

	// Exit current process since we ran an elevated one
	os.Exit(0)
	return nil
}

func getUserPrivilegeInfoUnix() string {
	user := os.Getenv("USER")
	uid := os.Getuid()
	gid := os.Getgid()
	euid := os.Geteuid()
	egid := os.Getegid()

	return fmt.Sprintf("User: %s, UID: %d, GID: %d, EUID: %d, EGID: %d",
		user, uid, gid, euid, egid)
}
