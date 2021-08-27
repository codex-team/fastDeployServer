package main

import (
	"context"
	"flag"
	"fmt"
	hawk "github.com/codex-team/hawk.go"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/n0str/restrictedflags"
	log "github.com/sirupsen/logrus"
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
var hawkAccessToken = flag.String("token", "", "Hawk access token")

var composeFilepaths arrayFlags
var configs []DockerComposeConfig

var dockerClient *client.Client
var hawkCatcher *hawk.Catcher

func main() {
	logLevel := restrictedflags.New([]string{"panic", "fatal", "error", "warn", "info", "debug", "trace"})
	flag.Var(&logLevel, "level", fmt.Sprintf("logging level (allowed: %s)", logLevel.AllowedValues))
	flag.Var(&composeFilepaths, "f", "docker-compose configuration path")
	flag.Parse()

	var err error

	options := hawk.DefaultHawkOptions()
	options.AccessToken = *hawkAccessToken
	options.Debug = logLevel.Value == "debug" || logLevel.Value == "trace"
	options.Transport = hawk.HTTPTransport{}
	options.Release = VERSION

	hawkCatcher, err = hawk.New(options)
	if err != nil {
		log.Fatalf("cannot initialize Hawk Catcher: %s", err)
	}

	go func() {
		_ = hawkCatcher.Run()
	}()
	defer hawkCatcher.Stop()

	log.SetLevel(getLogLevel(logLevel.Value))
	if len(composeFilepaths) == 0 {
		composeFilepaths = []string{"docker-compose.yml"}
	}

	dockerClient, err = client.NewClientWithOpts()
	if err != nil {
		panic(fmt.Sprintf("unable to create docker client: %s", err))
	}

	// initial configuration load
	for _, file := range composeFilepaths {
		var config DockerComposeConfig
		log.Infof("load %s configuration", file)
		if err = config.parse(file); err == nil {
			configs = append(configs, config)
		}
	}

	var wg sync.WaitGroup
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		for {
			wg.Add(2)

			ticker := time.NewTicker(*interval)

			go func() {
				log.Debugf("new sync interval")
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
	log.Infof("stopped")
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

	for key := range uniqueImagesSet {
		uniqueImagesList = append(uniqueImagesList, key)
	}
	return uniqueImagesList
}

// refreshImages - update all used images and return those been updated
func refreshImages(configs []DockerComposeConfig) map[string]struct{} {
	uniqueImagesList := getUniqueImages(configs)
	log.Debugf("extracted unique images: %s", uniqueImagesList)
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

	// return if there is nothing to update
	if len(images) == 0 {
		return
	}

	log.Infof("images to be pulled from registry: %s", images)

	if err := restartServices(configs, images); err != nil {
		log.Errorf("error during restartServices: %s", err)
		hawkCatcher.Catch(err, hawk.WithContext(images))
	}
}

// restartServices - restart services from compose config which use updatedImages
func restartServices(configs []DockerComposeConfig, updatedImages map[string]struct{}) error {
	// get list of all running containers
	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		hawkCatcher.Catch(err, hawk.WithContext(struct {
			updatedImages map[string]struct{}
		}{updatedImages: updatedImages}))
		return fmt.Errorf("unable to list docker containers: %s", err)
	}

	if len(containers) == 0 {
		log.Debugf("there are no containers running\n")
	}

	var stoppedContainers []string

	// stop each container with image equals targetImageName
	for _, container := range containers {
		if len(container.Names) == 0 {
			log.Debugf("Container %s has no names\n", container.ID)
			continue
		}

		containerName := container.Names[0]
		log.Debugf("checking container: %s %s %s\n", container.ID, container.Image, container.Names)
		if _, ok := updatedImages[container.Image]; ok {
			log.Infof("[>] stopping %s because of %s ...", container.ID, container.Image)

			if err := dockerClient.ContainerStop(context.Background(), container.ID, nil); err != nil {
				log.Warnf("unable to stop container %s: %s\n", container.ID, err)
				continue
			}

			log.Infof("container stopped\n")
			stoppedContainers = append(stoppedContainers, containerName)
		}
	}

	log.Infof("stopped %d containers", len(stoppedContainers))

	//iterate docker-compose files on watch
	for _, config := range configs {
		// reload config each time to monitor changes
		config.reload()
		log.Infof("starting containers from %s", config.Filename)
		// iterate services in each docker-compose file
		for serviceName, serviceData := range config.Services {
			if _, ok := updatedImages[serviceData.Image]; ok {
				log.Infof("  [>] starting %s because of %s ...\n", serviceName, serviceData.Image)
				_, err := exec.Command("docker-compose", "-f", config.Filename, "up", "-d", "--no-deps", serviceName).Output()
				if err != nil {
					log.Warnf("  [x] Unable to start %s: %s\n", serviceName, err)
					continue
				}

				log.Infof("  [+] Done.\n")
			}
		}
	}

	// notify via CodeX Bot
	if *codexBotURL != "" && len(updatedImages) > 0 {
		// prepare a list of updated images for notification
		updatedImagesList := make([]string, 0, len(updatedImages))
		for key := range updatedImages {
			updatedImagesList = append(updatedImagesList, key)
		}

		data := url.Values{}
		data.Set("message", fmt.Sprintf("📦 %s has been deployed: %s", *serverName, strings.Join(updatedImagesList, ", ")))

		_, err := MakeHTTPRequest("POST", *codexBotURL, []byte(data.Encode()), map[string]string{
			"Content-Type": "application/x-www-form-urlencoded",
		})
		if err != nil {
			hawkCatcher.Catch(err, hawk.WithContext(struct {
				message string
			}{message: data.Encode()}))
			return fmt.Errorf("Webhook error: %v", err)
		}
	}

	log.Debugf("[+] Done execution")
	return nil
}
