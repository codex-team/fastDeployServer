package main

import (
	"fmt"
	"github.com/docker/docker/client"
	log "github.com/sirupsen/logrus"
	"strings"
	"testing"
)

func setup() {
	var err error
	dockerClient, err = client.NewClientWithOpts()
	if err != nil {
		log.Fatalf("unable to create docker client: %s", err)
	}
}

// integration check that pullAndCheckImageHasUpdates can search for newer image properly
func Test_pullAndCheckImageHasUpdates(t *testing.T) {
	setup()
	deleteImage("codexteamuser/hawk-collector:prod")

	assert(t, pullAndCheckImageHasUpdates("docker.io/codexteamuser/hawk-collector:prod", creds), "newer image not found")
	assert(t, pullAndCheckImageHasUpdates("docker.io/codexteamuser/hawk-collector:prod", creds) == false, "image must already exist")
}

// check that images are correctly parsed from dockerfiles
func Test_uniqueImagesOfDockerConfig(t *testing.T) {
	setup()
	var configs = []DockerComposeConfig{{}, {}}
	err1 := configs[0].parse("tests/docker-compose-1.yml")
	err2 := configs[1].parse("tests/docker-compose-2.yml")
	err3 := configs[1].parse("tests/docker-compose-3.yml")

	log.Printf("!! %s", err3.Error())

	assert(t, err1 == nil, fmt.Sprintf("docker-compose-1 was parsed with a error: %s", err1))
	assert(t, err2 == nil, fmt.Sprintf("docker-compose-2 was parsed with a error: %s", err2))
	assert(t, strings.Contains(err3.Error(), "open tests/docker-compose-3.yml: no such file or directory"), "docker-compose-3 parse error invalid")
	assert(t, testUnorderedEq(getUniqueImages(configs), []string{"codexteamuser/hawk-collector:prod", "redis:6.0.9", "codexteamuser/hawk-garage:prod"}), "invalid values of unique images")
}

// integration check that images are correctly pulled are services restart
func Test_imagesRefreshAndRestart(t *testing.T) {
	setup()
	var configs = []DockerComposeConfig{{}, {}}
	_ = configs[0].parse("tests/docker-compose-1.yml")
	_ = configs[1].parse("tests/docker-compose-2.yml")
	images := refreshImages(configs, creds)
	_ = restartServices(configs, images)
}
