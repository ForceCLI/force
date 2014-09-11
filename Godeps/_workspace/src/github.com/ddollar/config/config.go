// Cross-platform configuration manager
package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
)

type Config struct {
	Base    string
	Entries map[string]ConfigEntry
}

type ConfigEntry struct {
	Key   string
	Value string
}

// Create a new Config manager
func NewConfig(base string) (config *Config) {
	config = &Config{}
	config.Base = base
	config.Entries = make(map[string]ConfigEntry)
	return
}

// List keys for a given config
func (c *Config) List(name string) (keys []string, err error) {
	dir := path.Join(c.configDirectory(), name)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return
	}
	for _, file := range files {
		keys = append(keys, file.Name())
	}
	sort.Strings(keys)
	return
}

// Save a key/value pair for a config
func (c *Config) Save(name, key, value string) (err error) {
	filename := path.Join(c.configDirectory(), name, key)
	err = c.writeFile(filename, value)
	return
}

// Load a value for a config key
func (c *Config) Load(name, key string) (body string, err error) {
	filename := path.Join(c.configDirectory(), name, key)
	body, err = c.readFile(filename)
	return
}

// Delete a config key/value pair
func (c *Config) Delete(name, key string) (err error) {
	filename := path.Join(c.configDirectory(), name, key)
	err = os.Remove(filename)
	return
}

func (c *Config) configDirectory() string {
	return path.Join(c.homeDirectory(), fmt.Sprintf(".%s", c.Base))
}

func (c *Config) writeFile(filename, body string) (err error) {
	dir := filepath.Dir(filename)
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	err = ioutil.WriteFile(filename, []byte(body), 0600)
	if err != nil {
		return
	}
	return
}

func (c *Config) readFile(filename string) (body string, err error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	body = string(data)
	return
}
