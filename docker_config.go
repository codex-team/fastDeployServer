package main

import (
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// Docker Compose v3 configuration
type DockerComposeConfig struct {
	Filename string
	Version  string                          `yaml:"version"`
	Volumes  map[string]interface{}          `yaml:"volumes"`
	Networks map[string]interface{}          `yaml:"networks"`
	Services map[string]DockerComposeService `yaml:"services"`
}

// Docker Compose v3 service settings in YAML
type DockerComposeService struct {
	Image string `yaml:"image"`
}

// Load DockerComposeConfig from filename
func (c *DockerComposeConfig) reload() {
	_ = c.parse(c.Filename)
}

// Load DockerComposeConfig from filename
func (c *DockerComposeConfig) parse(filepath string) error {
	c.Filename = filepath

	yamlFile, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Errorf("yaml file get error: #%v ", err)
		_ = hawkCatcher.Catch(err)
		return err
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Errorf("yaml unmarshal error: %v", err)
		_ = hawkCatcher.Catch(err)
		return err
	}

	log.Debugf("[+] successfully loaded configuration from %s\n", filepath)
	return nil
}
