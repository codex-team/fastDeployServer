package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var codexBotURL = flag.String("webhook", "", "notification URI from CodeX Bot")
var interval = flag.Duration("interval", 15*time.Second, "server name")
var serverName = flag.String("name", "default", "server name")

var composeFilepaths arrayFlags
var configs []DockerComposeConfig

func main() {
	flag.Var(&composeFilepaths, "f", "docker-compose configuration path")
	flag.Parse()
	if len(composeFilepaths) == 0 {
		composeFilepaths = []string{"docker-compose.yml"}
	}

	for _, file := range composeFilepaths {
		var config DockerComposeConfig
		config.parse(file)
		configs = append(configs, config)
	}

	var wg sync.WaitGroup
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			wg.Add(2)

			ticker := time.NewTicker(*interval)

			go func() {
				updateAndRestart()
				defer wg.Done()
			}()

			go func() {
				<-ticker.C
				defer wg.Done()
			}()

			wg.Wait()
		}
	}()

	<-done
	log.Printf("Stopped\n")
}

// getUniqueImages - parse compose configs and extract unique used images
func getUniqueImages(configs []DockerComposeConfig) []string {
	uniqueImagesSet := make(map[string]struct{})
	for _, config := range configs {
		for _, serviceData := range config.Services {
			if _, ok := uniqueImagesSet[serviceData.Image]; !ok {
				uniqueImagesSet[serviceData.Image] = struct{}{}
			}
		}
	}
	uniqueImagesList := make([]string, 0, len(uniqueImagesSet))

	for key, _ := range uniqueImagesSet {
		uniqueImagesList = append(uniqueImagesList, key)
	}
	return uniqueImagesList
}

// refreshImages - update all used images and return those been updated
func refreshImages(configs []DockerComposeConfig) map[string]struct{} {
	uniqueImagesList := getUniqueImages(configs)
	updatedImages := make(map[string]struct{})

	for _, image := range uniqueImagesList {
		if isUpdated := pullAndCheckImageHasUpdates(fmt.Sprintf("docker.io/%s", image)); isUpdated {
			updatedImages[image] = struct{}{}
		}
	}

	return updatedImages
}

// updateAndRestart - update images and restart compose services which use these images
func updateAndRestart() {
	images := refreshImages(configs)
	if err := restartServices(configs, images); err != nil {
		log.Fatalf("Fatal error: %s", err)
	}
}

// restartServices - restart services from compose config which use updatedImages
func restartServices(configs []DockerComposeConfig, updatedImages map[string]struct{}) error {
	cli, err := client.NewClientWithOpts()
	if err != nil {
		return fmt.Errorf("unable to create docker client: %s", err)
	}
	// get list of all running containers
	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return fmt.Errorf("Unable to list docker containers: %s", err)
	}

	if len(containers) == 0 {
		log.Printf("there are no containers running\n")
	}

	var stoppedContainers []string

	// stop each container with image equals targetImageName
	for _, container := range containers {
		if len(container.Names) == 0 {
			log.Printf("Container %s has no names\n", container.ID)
			continue
		}

		containerName := container.Names[0]
		log.Printf("Container: %s %s %s\n", container.ID, container.Image, container.Names)
		if _, ok := updatedImages[container.Image]; ok {
			log.Printf("  [>] stopping %s because of %s ...", container.ID, container.Image)

			if err := cli.ContainerStop(context.Background(), container.ID, nil); err != nil {
				log.Printf("  [x] Unable to stop container %s: %s\n", container.ID, err)
				continue
			}

			log.Printf("  [+] Done.\n")
			stoppedContainers = append(stoppedContainers, containerName)
		}
	}

	log.Printf("[!] Stopped %d containers", len(stoppedContainers))

	//iterate docker-compose files on watch
	for _, config := range configs {
		log.Printf("Starting containers from %s", config.Filename)
		// iterate services in each docker-compose file
		for serviceName, serviceData := range config.Services {
			if _, ok := updatedImages[serviceData.Image]; ok {
				//fmt.Printf("Need to update %s because of %s", serviceName, serviceData.Image)
				log.Printf("  [>] starting %s because of %s ...\n", serviceName, serviceData.Image)
				_, err := exec.Command("docker-compose", "-f", config.Filename, "up", "-d", "--no-deps", serviceName).Output()
				if err != nil {
					log.Printf("  [x] Unable to start %s: %s\n", serviceName, err)
					continue
				}

				log.Printf("  [+] Done.\n")
			}
		}
	}

	// notify via CodeX Bot
	if *codexBotURL != "" && len(updatedImages) > 0 {
		// prepare a list of updated images for notification
		updatedImagesList := make([]string, 0, len(updatedImages))
		for key, _ := range updatedImages {
			updatedImagesList = append(updatedImagesList, key)
		}

		data := url.Values{}
		data.Set("message", fmt.Sprintf("📦 %s has been deployed: %s", *serverName, strings.Join(updatedImagesList, ", ")))

		_, err := MakeHTTPRequest("POST", *codexBotURL, []byte(data.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		})
		if err != nil {
			return fmt.Errorf("Webhook error: %v", err)
		}
	}

	log.Printf("[+] Done execution")
	return nil
}
