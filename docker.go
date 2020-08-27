package main

import (
	"io/ioutil"
	"log"
	"strings"

	"gopkg.in/yaml.v2"
)

type DockerComposeConfig struct {
	Filename string
	Version  string                          `yaml:"version"`
	Volumes  map[string]interface{}          `yaml:"volumes"`
	Networks map[string]interface{}          `yaml:"networks"`
	Services map[string]DockerComposeService `yaml:"services"`
}

type DockerComposeService struct {
	Image string `yaml:"image"`
}

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

func (c *DockerComposeConfig) findServiceToUp(targetImageName, targetContainerName string) string {
	for serviceName, serviceData := range c.Services {
		if serviceData.Image == targetImageName && strings.Contains(targetContainerName, serviceName) {
			return serviceName
		}
	}
	return ""
}

func (c *DockerComposeConfig) findServicesToUp(targetImageName string) []string {
	var servicesToUp []string
	for serviceName, serviceData := range c.Services {
		if serviceData.Image == targetImageName {
			servicesToUp = append(servicesToUp, serviceName)
		}
	}
	return servicesToUp
}
