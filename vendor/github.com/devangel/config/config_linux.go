package config

import (
	"os"
)

func (c *Config) homeDirectory() string {
	return os.Getenv("HOME")
}
