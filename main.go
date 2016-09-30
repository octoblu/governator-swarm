package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/codegangsta/cli"
	"github.com/coreos/go-semver/semver"
	"github.com/docker/engine-api/client"
	"github.com/fatih/color"
	"github.com/garyburd/redigo/redis"
	"github.com/octoblu/governator-swarm/deployer"
	De "github.com/tj/go-debug"
)

var debug = De.Debug("governator-swarm:main")

func main() {
	app := cli.NewApp()
	app.Name = "governator-swarm"
	app.Version = version()
	app.Action = run
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "docker-uri, d",
			EnvVar: "GOVERNATOR_DOCKER_URI",
			Usage:  "Docker server to deploy to",
			Value:  "unix:///var/run/docker.sock",
		},
		cli.StringFlag{
			Name:   "redis-uri, r",
			EnvVar: "GOVERNATOR_REDIS_URI",
			Usage:  "Redis server to pull deployments from",
		},
		cli.StringFlag{
			Name:   "redis-queue, q",
			EnvVar: "GOVERNATOR_REDIS_QUEUE",
			Usage:  "Redis queue to pull deployments from",
		},
		cli.StringFlag{
			Name:   "deploy-state-uri",
			EnvVar: "DEPLOY_STATE_URI",
			Usage:  "Deploy state uri, it should include authentication.",
		},
		cli.StringFlag{
			Name:   "cluster",
			EnvVar: "CLUSTER",
			Usage:  "The current running cluster",
		},
	}
	app.Run(os.Args)
}

func run(context *cli.Context) {
	dockerURI, redisURI, redisQueue, deployStateURI, cluster := getOpts(context)

	dockerClient := getDockerClient(dockerURI)

	redisConn := getRedisConn(redisURI)

	theDeployer := deployer.New(dockerClient, redisConn, redisQueue, deployStateURI, cluster)
	sigTerm := make(chan os.Signal)
	signal.Notify(sigTerm, syscall.SIGTERM)

	sigTermReceived := false

	go func() {
		<-sigTerm
		fmt.Println("SIGTERM received, waiting to exit")
		sigTermReceived = true
	}()

	for {
		if sigTermReceived {
			fmt.Println("I'll be back.")
			os.Exit(0)
		}

		debug("theDeployer.Run()")
		err := theDeployer.Run()
		if err != nil {
			log.Panic("Run error", err)
		}
		time.Sleep(1 * time.Second)
	}
}

func getOpts(context *cli.Context) (string, string, string, string, string) {
	dockerURI := context.String("docker-uri")
	redisURI := context.String("redis-uri")
	redisQueue := context.String("redis-queue")
	deployStateURI := context.String("deploy-state-uri")
	cluster := context.String("cluster")

	if dockerURI == "" || redisURI == "" || redisQueue == "" || deployStateURI == "" || cluster == "" {
		cli.ShowAppHelp(context)

		if dockerURI == "" {
			color.Red("  Missing required flag --docker-uri or GOVERNATOR_DOCKER_URI")
		}
		if redisURI == "" {
			color.Red("  Missing required flag --redis-uri or GOVERNATOR_REDIS_URI")
		}
		if redisQueue == "" {
			color.Red("  Missing required flag --redis-queue or GOVERNATOR_REDIS_QUEUE")
		}
		if deployStateURI == "" {
			color.Red("  Missing required flag --deploy-state-uri or DEPLOY_STATE_URI")
		}
		if cluster == "" {
			color.Red("  Missing required flag --cluster or CLUSTER")
		}
		os.Exit(1)
	}

	return dockerURI, redisURI, redisQueue, deployStateURI, cluster
}

func getDockerClient(dockerURI string) client.APIClient {
	defaultHeaders := map[string]string{"User-Agent": "governator-swarm"}

	dockerClient, err := client.NewClient(dockerURI, "v1.24", nil, defaultHeaders)
	if err != nil {
		panic(err)
	}
	return dockerClient
}

func getRedisConn(redisURI string) redis.Conn {
	redisConn, err := redis.DialURL(redisURI)
	if err != nil {
		log.Panicln("Error with redis.DialURL", err.Error())
	}
	return redisConn
}

// ParseHost verifies that the given host strings is valid.
func ParseHost(host string) (string, string, string, error) {
	protoAddrParts := strings.SplitN(host, "://", 2)
	if len(protoAddrParts) == 1 {
		return "", "", "", fmt.Errorf("unable to parse docker host `%s`", host)
	}

	var basePath string
	proto, addr := protoAddrParts[0], protoAddrParts[1]
	if proto == "tcp" {
		parsed, err := url.Parse("tcp://" + addr)
		if err != nil {
			return "", "", "", err
		}
		addr = parsed.Host
		basePath = parsed.Path
	}
	return proto, addr, basePath, nil
}

func version() string {
	version, err := semver.NewVersion(VERSION)
	if err != nil {
		errorMessage := fmt.Sprintf("Error with version number: %v", VERSION)
		log.Panicln(errorMessage, err.Error())
	}
	return version.String()
}
