package deployer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/garyburd/redigo/redis"
	De "github.com/tj/go-debug"
)

var debug = De.Debug("governator:deployer")

// Deployer watches a redis queue
// and deploys services using Etcd
type Deployer struct {
	dockerClient   client.APIClient
	redisConn      redis.Conn
	queueName      string
	deployStateURI string
	cluster        string
}

// RequestMetadata is the metadata of the request
type RequestMetadata struct {
	EtcdDir   string `json:"etcdDir"`
	DockerURL string `json:"dockerUrl"`
}

// New constructs a new deployer instance
func New(dockerClient client.APIClient, redisConn redis.Conn, queueName, deployStateURI, cluster string) *Deployer {
	return &Deployer{
		dockerClient:   dockerClient,
		redisConn:      redisConn,
		queueName:      queueName,
		deployStateURI: deployStateURI,
		cluster:        cluster,
	}
}

// Run watches the redis queue and starts taking action
func (deployer *Deployer) Run() error {
	deploy, err := deployer.getNextValidDeploy()
	if err != nil {
		return err
	}

	if deploy == nil {
		return nil
	}

	return deployer.deploy(deploy)
}

func (deployer *Deployer) getReleaseVersion(dockerURL string) string {
	parts := strings.Split(dockerURL, ":")
	return parts[len(parts)-1]
}

func (deployer *Deployer) getKey(key string) string {
	return fmt.Sprintf("%s:%s", deployer.queueName, key)
}

func (deployer *Deployer) deploy(metadata *RequestMetadata) error {
	var err error
	dockerClient := deployer.dockerClient

	_, repo, _ := deployer.parseDockerURL(metadata.DockerURL)

	ctx := context.Background()
	updateOpts := types.ServiceUpdateOptions{}

	service, _, err := dockerClient.ServiceInspectWithRaw(ctx, repo)
	if err != nil {
		return err
	}

	service.Spec.TaskTemplate.ContainerSpec.Image = metadata.DockerURL

	err = dockerClient.ServiceUpdate(ctx, service.ID, service.Version, service.Spec, updateOpts)
	if err != nil {
		return err
	}

	err = deployer.notifyDeployState(metadata.DockerURL)
	if err != nil {
		return err
	}

	return nil
}

func (deployer *Deployer) getNextDeploy() (string, error) {
	now := time.Now().Unix()
	deploysResult, err := deployer.redisConn.Do("ZRANGEBYSCORE", deployer.getKey("governator:deploys"), 0, now)

	if err != nil {
		return "", err
	}

	deploys := deploysResult.([]interface{})
	if len(deploys) == 0 {
		return "", nil
	}

	return string(deploys[0].([]byte)), nil
}

func (deployer *Deployer) lockDeploy(deploy string) (bool, error) {
	debug("lockDeploy: %v", deploy)
	zremResult, err := deployer.redisConn.Do("ZREM", deployer.getKey("governator:deploys"), deploy)

	if err != nil {
		return false, err
	}

	result := zremResult.(int64)

	return (result != 0), nil
}

func (deployer *Deployer) validateDeploy(deploy string) (bool, error) {
	debug("validateDeploy: %v", deploy)
	existsResult, err := deployer.redisConn.Do("HEXISTS", deployer.getKey(deploy), "cancellation")

	if err != nil {
		return false, err
	}

	exists := existsResult.(int64)
	return (exists == 0), nil
}

func (deployer *Deployer) getMetadata(deploy string) (*RequestMetadata, error) {
	debug("getMetadata: %v", deploy)
	var metadata RequestMetadata

	metadataBytes, err := deployer.redisConn.Do("HGET", deployer.getKey(deploy), "request:metadata")
	if err != nil {
		return nil, err
	}

	if metadataBytes == nil {
		return nil, fmt.Errorf("Deploy metadata not found for '%v'", deploy)
	}

	err = json.Unmarshal(metadataBytes.([]byte), &metadata)

	if err != nil {
		return nil, err
	}

	return &metadata, nil
}

func (deployer *Deployer) getNextValidDeploy() (*RequestMetadata, error) {
	deploy, err := deployer.getNextDeploy()
	if err != nil {
		return nil, err
	}

	if deploy == "" {
		return nil, nil
	}

	ok, err := deployer.lockDeploy(deploy)
	if err != nil {
		return nil, err
	}

	if !ok {
		debug("Failed to obtain lock for: %v", deploy)
		return nil, nil
	}

	ok, err = deployer.validateDeploy(deploy)
	if err != nil {
		return nil, err
	}

	if !ok {
		debug("Deploy was cancelled: %v", deploy)
		return nil, nil
	}

	return deployer.getMetadata(deploy)
}

func (deployer *Deployer) parseDockerURL(dockerURL string) (string, string, string) {
	var owner, repo, tag string
	dockerURLParts := strings.Split(dockerURL, ":")

	if len(dockerURLParts) != 2 {
		return "", "", ""
	}

	if dockerURLParts[1] != "" {
		tag = dockerURLParts[1]
	}

	projectParts := strings.Split(dockerURLParts[0], "/")

	if len(projectParts) == 2 {
		owner = projectParts[0]
		repo = projectParts[1]
	} else if len(projectParts) == 3 {
		owner = projectParts[1]
		repo = projectParts[2]
	} else {
		return "", "", ""
	}

	return owner, repo, tag
}

func (deployer *Deployer) notifyDeployState(dockerURL string) error {
	owner, repo, tag := deployer.parseDockerURL(dockerURL)

	uri := fmt.Sprintf("deployments/%s/%s/%s/cluster/%s/passed", owner, repo, tag, deployer.cluster)
	fullURL := fmt.Sprintf("%s/%s", deployer.deployStateURI, uri)

	debug("making request to %s", fullURL)
	client := &http.Client{}
	request, err := http.NewRequest("PUT", fullURL, nil)
	if err != nil {
		return err
	}
	response, err := client.Do(request)
	if err != nil {
		return err
	}
	debug("Response StatusCode %v", response.StatusCode)

	response.Body.Close()
	if response.StatusCode > 399 {
		return errors.New("invalid response from deploy-state-service")
	}
	return nil
}
