# Fast Deploy Server

Helps to simply pull and restart docker-compose images

## Usage

FastDeployServer continuously monitor multiple docker-compose configurations 
and repitedly check if some of services use outdated images.
FastDeployServer will pull images from remote repository, stop Docker containers that 
use these images and restart docker-compose services that use these images.

```
Usage of ./fastDeployServer:
  -f value
        docker-compose configuration path
  -interval duration
        server name (default 15s)
  -name string
        server name (default "default")
  -webhook string
        notification URI from CodeX Bot
```

### Example

Say you have the following docker-compose.yml configuration
``` 
version: "3.2"
services:
  collector:
    image: codexteamuser/hawk-collector:prod
    restart: unless-stopped

  garage:
    image: codexteamuser/hawk-garage:prod
    restart: unless-stopped
```

and you want to restart `hawk-collector` and `hawk-garage` services if there is an updated image 
`codexteamuser/hawk-collector:prod` or `codexteamuser/hawk-garage:prod`.

Just run the FastDeployServer `./fastDeployServer -f docker-compose.yml` and it will start 
checking pulling image updates every 20 seconds.

* You can configure interval as `./fastDeployServer -f docker-compose.yml -interval 5m`
* You can get notification via CodeX Bot as `./fastDeployServer -f docker-compose.yml -webhook https://notify.bot.codex.so/u/<...>`
* You can vary bot notification with `-name myserver` and get message `📦 myserver has been deployed: codexteamuser/hawk-garage:prod`

## Build

To build binary for Linux, run and then copy `fastDeployServer` binary wherever you want
```
GOOS=linux GOARCH=amd64 go build .
```


