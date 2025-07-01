package core

import "runtime"

const (
	Platform = runtime.GOOS
)

func IsWindows() bool {
	return Platform == "windows"
}

func IsMacOS() bool {
	return Platform == "darwin"
}
