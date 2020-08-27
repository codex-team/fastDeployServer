package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var listenAddr = flag.String("addr", "localhost:8090", "listen host and port")
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

	http.HandleFunc("/", restartHandler)
	http.ListenAndServe(*listenAddr, nil)
}

func restartHandler(w http.ResponseWriter, req *http.Request) {
	keys, ok := req.URL.Query()["image"]
	if !ok || len(keys[0]) < 1 {
		log.Printf("Url Param 'image' is missing\n")
		return
	}
	image := keys[0]
	restart(image)
}

func restart(targetImageName string) {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("Unable to create docker client: %s", err)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Fatalf("Unable to list docker containers: %s", err)
	}

	if len(containers) == 0 {
		log.Printf("There are no containers running")
		return
	}

	var stoppedContainers []string

	for _, container := range containers {
		if len(container.Names) == 0 {
			log.Printf("Container %s has no names", container.ID)
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

	log.Printf("[>] pulling %s ...\n", targetImageName)

	_, err = exec.Command("docker", "pull", targetImageName).Output()
	if err != nil {
		log.Printf("[x] Unable to pull %s: %s\n", targetImageName, err)
	}

	for _, config := range configs {
		servicesToUp := config.findServicesToUp(targetImageName)
		if len(servicesToUp) == 0 {
			continue
		}

		log.Printf("Going to run services from '%s':\n  - %s\n", config.Filename, strings.Join(servicesToUp, "\n  - "))
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

	log.Printf("[+] Done execution")

}
