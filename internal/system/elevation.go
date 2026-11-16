package system

// ElevationManager handles checking and requesting administrator/root privileges.
type ElevationManager interface {
	IsElevated() bool
	RunElevated(executable string, args []string) error
}
