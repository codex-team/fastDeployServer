package main

import (
	"testing"
)

// integration check that pullAndCheckImageHasUpdates can search for newer image properly
func Test_pullAndCheckImageHasUpdates(t *testing.T) {
	deleteImage("codexteamuser/hawk-collector:prod")

	assert(t, pullAndCheckImageHasUpdates("docker.io/codexteamuser/hawk-collector:prod"), "newer image not found")
	assert(t, pullAndCheckImageHasUpdates("docker.io/codexteamuser/hawk-collector:prod") == false, "image must already exist")
}

// check that images are correctly parsed from dockerfiles
func Test_uniqueImagesOfDockerConfig(t *testing.T) {
	var configs = []DockerComposeConfig{{}, {}}
	configs[0].parse("tests/docker-compose-1.yml")
	configs[1].parse("tests/docker-compose-2.yml")
	assert(t, testUnorderedEq(getUniqueImages(configs), []string{"codexteamuser/hawk-collector:prod", "redis:6.0.9", "codexteamuser/hawk-garage:prod"}), "invalid values of unique images")
}

// integration check that images are correctly pulled are services restart
func Test_imagesRefreshAndRestart(t *testing.T) {
	var configs = []DockerComposeConfig{{}, {}}
	configs[0].parse("tests/docker-compose-1.yml")
	configs[1].parse("tests/docker-compose-2.yml")
	images := refreshImages(configs)
	restartServices(configs, images)
}