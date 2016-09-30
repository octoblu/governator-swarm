package etcdclient

import (
	"fmt"
	"time"

	"github.com/coreos/etcd/client"
	"golang.org/x/net/context"
)

// EtcdClient interface lets your Get/Set from Etcd
type EtcdClient interface {
	// Del deletes a key from Etcd
	Del(key string) error

	// DelDir deletes a dir from Etcd
	DelDir(key string) error

	// Get gets a value in Etcd
	Get(key string) (string, error)

	// Set sets a value in Etcd
	Set(key, value string) error

	// UpdateDirWithTTL updates a directory with a ttl value
	UpdateDirWithTTL(key string, ttl time.Duration) error

	// Ls returns all the keys available in the directory
	Ls(directory string) ([]string, error)

	// LsRecursive returns all the keys available in the directory, recursively
	LsRecursive(directory string) ([]string, error)

	// MkDir creates an empty etcd directory
	MkDir(directory string) error

	// WatchRecursive watches a directory and calls the callback everytime something changes.
	// The callback is called with the key of the thing that changed along with the value
	// that the thing was changed to.
	// This method only returns if there is an error
	WatchRecursive(directory string, onChangeCallback OnChangeCallback) error
}

// OnChangeCallback is used for passing callbacks to
// WatchRecursive
type OnChangeCallback func(key, newValue string)

// SimpleEtcdClient implements EtcdClient
type SimpleEtcdClient struct {
	etcd client.Client
}

// Dial constructs a new EtcdClient
func Dial(etcdURI string) (EtcdClient, error) {
	etcd, err := client.New(client.Config{
		Endpoints: []string{etcdURI},
	})
	if err != nil {
		return nil, err
	}
	return &SimpleEtcdClient{etcd}, nil
}

// Del deletes a key from Etcd
func (etcdClient *SimpleEtcdClient) Del(key string) error {
	api := client.NewKeysAPI(etcdClient.etcd)
	_, err := api.Delete(context.Background(), key, nil)
	if err != nil {
		if client.IsKeyNotFound(err) {
			return nil
		}
	}
	return err
}

// DelDir deletes a dir from Etcd
func (etcdClient *SimpleEtcdClient) DelDir(key string) error {
	api := client.NewKeysAPI(etcdClient.etcd)
	_, err := api.Delete(context.Background(), key, &client.DeleteOptions{Dir: true, Recursive: true})
	if err != nil {
		if client.IsKeyNotFound(err) {
			return nil
		}
	}
	return err
}

// Get gets a value in Etcd
func (etcdClient *SimpleEtcdClient) Get(key string) (string, error) {
	api := client.NewKeysAPI(etcdClient.etcd)
	response, err := api.Get(context.Background(), key, nil)
	if err != nil {
		if client.IsKeyNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return response.Node.Value, nil
}

// Set sets a value in Etcd
func (etcdClient *SimpleEtcdClient) Set(key, value string) error {
	api := client.NewKeysAPI(etcdClient.etcd)
	_, err := api.Set(context.Background(), key, value, nil)
	return err
}

// UpdateDirWithTTL updates a directory with a ttl value
func (etcdClient *SimpleEtcdClient) UpdateDirWithTTL(key string, ttl time.Duration) error {
	api := client.NewKeysAPI(etcdClient.etcd)
	_, err := api.Set(context.Background(), key, "", &client.SetOptions{TTL: ttl, Dir: true, PrevExist: client.PrevExist})
	return err
}

// Ls returns all the keys available in the directory
func (etcdClient *SimpleEtcdClient) Ls(directory string) ([]string, error) {
	api := client.NewKeysAPI(etcdClient.etcd)
	options := &client.GetOptions{Sort: true, Recursive: false}
	response, err := api.Get(context.Background(), directory, options)

	if err != nil {
		if client.IsKeyNotFound(err) {
			return make([]string, 0), nil
		}
		return make([]string, 0), err
	}

	return nodesToStringSlice(response.Node.Nodes), nil
}

// LsRecursive returns all the keys available in the directory, recursively
func (etcdClient *SimpleEtcdClient) LsRecursive(directory string) ([]string, error) {
	api := client.NewKeysAPI(etcdClient.etcd)
	options := &client.GetOptions{Sort: true, Recursive: true}
	response, err := api.Get(context.Background(), directory, options)

	if err != nil {
		if client.IsKeyNotFound(err) {
			return make([]string, 0), nil
		}
		return make([]string, 0), err
	}

	return nodesToStringSlice(response.Node.Nodes), nil
}

// MkDir creates an empty etcd directory
func (etcdClient *SimpleEtcdClient) MkDir(directory string) error {
	api := client.NewKeysAPI(etcdClient.etcd)
	results, err := api.Get(context.Background(), directory, nil)

	if err != nil && !client.IsKeyNotFound(err) {
		return err
	}

	if err != nil && client.IsKeyNotFound(err) {
		_, err = api.Set(context.Background(), directory, "", &client.SetOptions{Dir: true, PrevExist: client.PrevIgnore})
		return err
	}

	if !results.Node.Dir {
		return fmt.Errorf("Refusing to overwrite key/value with a directory: %v", directory)
	}
	return nil
}

// WatchRecursive watches a directory and calls the callback everytime something changes.
// The callback is called with the key of the thing that changed along with the value
// that the thing was changed to.
// This method only returns if there is an error
func (etcdClient *SimpleEtcdClient) WatchRecursive(directory string, onChange OnChangeCallback) error {
	api := client.NewKeysAPI(etcdClient.etcd)
	afterIndex := uint64(0)

	for {
		watcher := api.Watcher(directory, &client.WatcherOptions{Recursive: true, AfterIndex: afterIndex})
		response, err := watcher.Next(context.Background())
		if err != nil {
			if shouldIgnoreError(err) {
				continue
			}
			return err
		}

		afterIndex = response.Index
		onChange(response.Node.Key, response.Node.Value)
	}
}

func nodesToStringSlice(nodes client.Nodes) []string {
	var keys []string

	for _, node := range nodes {
		keys = append(keys, node.Key)

		for _, key := range nodesToStringSlice(node.Nodes) {
			keys = append(keys, key)
		}
	}

	return keys
}

func shouldIgnoreError(err error) bool {
	switch err := err.(type) {
	default:
		return false
	case *client.Error:
		return err.Code == client.ErrorCodeEventIndexCleared
	}
}
