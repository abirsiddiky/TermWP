package system

// EnvManager coordinates user environment PATH configuration and system capabilities.
type EnvManager struct{}

// NewEnvManager creates an EnvManager.
func NewEnvManager() *EnvManager {
	return &EnvManager{}
}
