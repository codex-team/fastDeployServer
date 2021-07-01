package main

import (
	"io/ioutil"
	"log"

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
	c.parse(c.Filename)
}

// Load DockerComposeConfig from filename
func (c *DockerComposeConfig) parse(filepath string) {
	c.Filename = filepath

	yamlFile, err := ioutil.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Yaml file get error: #%v ", err)
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		log.Fatalf("Yaml unmarshal error: %v", err)
	}

	log.Printf("[+] Load configuration from %s\n", filepath)
}

// Find services names based on image with targetImageName
func (c *DockerComposeConfig) findServicesToUp(targetImageName string) []string {
	var servicesToUp []string
	for serviceName, serviceData := range c.Services {
		if serviceData.Image == targetImageName {
			servicesToUp = append(servicesToUp, serviceName)
		}
	}
	return servicesToUp
}
