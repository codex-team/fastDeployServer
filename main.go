package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
)

var listenAddr = flag.String("addr", "localhost:8090", "listen host and port")
var codexBotURL = flag.String("webhook", "", "notification URI from CodeX Bot")
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

	router := mux.NewRouter()
	router.HandleFunc("/", restartHandler).Methods("GET")

	server := &http.Server{
		Addr:    *listenAddr,
		Handler: router,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	<-done
	log.Printf("Server stopped\n")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer func() {
		cancel()
	}()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v\n", err)
	}
}

// restartHandler - restart docker containers by the image name provided via webhook
func restartHandler(w http.ResponseWriter, req *http.Request) {
	keys, ok := req.URL.Query()["image"]
	if !ok || len(keys[0]) < 1 {
		SendError(w, "url Param 'image' is missing", http.StatusBadRequest)
		return
	}
	image := keys[0]
	if err := restart(image); err != nil {
		SendError(w, fmt.Sprintf("Error: %s\n", err), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// restart - restart docker containers by the targetImageName
func restart(targetImageName string) error {
	cli, err := client.NewEnvClient()
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
		if container.Image == targetImageName {
			log.Printf("  [>] stopping %s ...", container.ID)

			if err := cli.ContainerStop(context.Background(), container.ID, nil); err != nil {
				log.Printf("  [x] Unable to stop container %s: %s\n", container.ID, err)
				continue
			}

			log.Printf("  [+] Done.\n")
			stoppedContainers = append(stoppedContainers, containerName)
		}
	}

	log.Printf("[!] Stopped %d containers", len(stoppedContainers))

	log.Printf("[>] pulling %s ...\n", targetImageName)

	// pull new version of image
	_, err = exec.Command("docker", "pull", targetImageName).Output()
	if err != nil {
		log.Printf("[x] Unable to pull %s: %s\n", targetImageName, err)
	}

	// start services in docker-compose files based on targetImageName
	for _, config := range configs {
		servicesToUp := config.findServicesToUp(targetImageName)
		if len(servicesToUp) == 0 {
			continue
		}

		log.Printf("Going to start services from '%s':\n  - %s\n", config.Filename, strings.Join(servicesToUp, "\n  - "))
		for _, serviceName := range servicesToUp {
			log.Printf("  [>] starting %s ...\n", serviceName)

			_, err := exec.Command("docker-compose", "-f", config.Filename, "up", "-d", "--no-deps", serviceName).Output()
			if err != nil {
				log.Printf("  [x] Unable to start %s: %s\n", serviceName, err)
				continue
			}

			log.Printf("  [+] Done.\n")
		}
	}

	// notify by CodeX Bot
	if *codexBotURL != "" {
		data := url.Values{}
		data.Set("message", fmt.Sprintf("📦 %s has been deployed (%s)", *serverName, targetImageName))

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
