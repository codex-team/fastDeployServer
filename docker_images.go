package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"io"
	"log"
	"strings"
)

// Docker API event
type DockerEvent struct {
	Status         string `json:"status"`
	Error          string `json:"error"`
	Progress       string `json:"progress"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}

// deleteImage - remove image by target name
func deleteImage(targetImageName string) {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("unable to create docker client: %s", err)
	}
	cli.ImageRemove(context.Background(), targetImageName, types.ImageRemoveOptions{})
}

// pullAndCheckImageHasUpdates - check whether the targetImageName has updates via pulling the image from remote repository
func pullAndCheckImageHasUpdates(targetImageName string) bool {
	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("unable to create docker client: %s", err)
	}

	// try to pull image
	events, err := cli.ImagePull(context.Background(), targetImageName, types.ImagePullOptions{})
	if err != nil {
		log.Fatalf("Unable to list pull image %s: %s", targetImageName, err)
	}

	d := json.NewDecoder(events)

	var event *DockerEvent
	for {
		if err := d.Decode(&event); err != nil {
			// last event
			if err == io.EOF {
				break
			}

			log.Fatalf("Fatal error during docker API event decoding: %s", err)
		}
	}

	// Sample latest event for new image
	// EVENT: {Status:Status: Downloaded newer image for busybox:latest Error: Progress:[==================================================>]  699.2kB/699.2kB ProgressDetail:{Current:699243 Total:699243}}
	// Sample latest event for up-to-date image
	// EVENT: {Status:Status: Image is up to date for busybox:latest Error: Progress: ProgressDetail:{Current:0 Total:0}}
	if event != nil {
		if strings.Contains(event.Status, fmt.Sprintf("Downloaded newer image for")) {
			return true
		}

		if strings.Contains(event.Status, fmt.Sprintf("Image is up to date for")) {
			return false
		}
	}

	log.Fatalf("Unexpected latest event: %v", event)
	return false
}