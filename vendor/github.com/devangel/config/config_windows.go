package config

import (
	"os"
	"path"
)

func (c *Config) homeDirectory() string {
	home := path.Join(os.Getenv("HOMEDRIVE"), os.Getenv("HOMEPATH"))
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	return home
}
