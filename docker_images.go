package main

import (
	"context"
	"encoding/json"
	"github.com/codex-team/hawk.go"
	"github.com/docker/docker/api/types"
	log "github.com/sirupsen/logrus"
	"io"
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
	if _, err := dockerClient.ImageRemove(context.Background(), targetImageName, types.ImageRemoveOptions{}); err != nil {
		log.Errorf("Cannot remove image: %s", err)
		hawkCatcher.Catch(err)
	}
}

// pullAndCheckImageHasUpdates - check whether the targetImageName has updates via pulling the image from remote repository
func pullAndCheckImageHasUpdates(targetImageName string) bool {
	// try to pull image
	events, err := dockerClient.ImagePull(context.Background(), targetImageName, types.ImagePullOptions{})
	if err != nil {
		log.Errorf("unable to list pull image %s: %s", targetImageName, err)
		hawkCatcher.Catch(err)
		return false
	}

	d := json.NewDecoder(events)

	var event *DockerEvent
	for {
		if err := d.Decode(&event); err != nil {
			// last event
			if err == io.EOF {
				break
			}

			log.Errorf("error during docker API event decoding: %s", err)
			hawkCatcher.Catch(err, hawk.WithContext(events))
			return false
		}
	}

	log.Debugf("last API event status: %s", event.Status)

	// Sample latest event for new image
	// EVENT: {Status:Status: Downloaded newer image for busybox:latest Error: Progress:[==================================================>]  699.2kB/699.2kB ProgressDetail:{Current:699243 Total:699243}}
	// Sample latest event for up-to-date image
	// EVENT: {Status:Status: Image is up to date for busybox:latest Error: Progress: ProgressDetail:{Current:0 Total:0}}
	if event != nil {
		if strings.Contains(event.Status, "Downloaded newer image for") {
			return true
		}

		if strings.Contains(event.Status, "Image is up to date for") {
			return false
		}
	}

	log.Errorf("unexpected latest event: %v", event)
	hawkCatcher.Catch(err)
	return false
}