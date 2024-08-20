package provider

import (
	"path/filepath"

	"github.com/loft-sh/devpod/pkg/config"
)

func GetLocksDir(service string) (string, error) {
	configDir, err := config.GetConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, service, "locks"), nil
}
