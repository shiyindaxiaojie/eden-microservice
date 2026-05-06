// Package naming_client is a drop-in replacement for
// github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client.
// It provides INamingClient backed by the local registry.
package naming_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	consulcompat "github.com/shiyindaxiaojie/eden-registry/internal/adapter/consul/compat"
	nacoscompat "github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/compat"
	nacosmodel "github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/model"
	"github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos/vo"
)

// INamingClient is the Nacos naming client interface.
type INamingClient interface {
	RegisterInstance(param vo.RegisterInstanceParam) (bool, error)
	DeregisterInstance(param vo.DeregisterInstanceParam) (bool, error)
	GetService(param vo.GetServiceParam) (nacosmodel.ServiceInfo, error)
	SelectAllInstances(param vo.SelectAllInstancesParam) ([]nacosmodel.Instance, error)
	SelectInstances(param vo.SelectInstancesParam) ([]nacosmodel.Instance, error)
	SelectOneHealthyInstance(param vo.SelectOneHealthyInstanceParam) (*nacosmodel.Instance, error)
	Subscribe(param *vo.SubscribeParam) error
	Unsubscribe(param *vo.SubscribeParam) error
	GetAllServicesInfo(param vo.GetAllServiceInfoParam) (nacosmodel.ServiceList, error)
}

// NamingClient implements INamingClient by calling the local registry HTTP API.
type NamingClient struct {
	registryAddr string
	client       *http.Client
	stopCh       chan struct{}
	mu           sync.Mutex
	subs         map[string]chan struct{} // service -> stop channel
}

// NewNamingClient creates a new naming client from Nacos params.
func NewNamingClient(param vo.NacosClientParam) (INamingClient, error) {
	if len(param.ServerConfigs) == 0 {
		return nil, fmt.Errorf("nacos-compat: at least one server config required")
	}

	sc := param.ServerConfigs[0]
	scheme := sc.Scheme
	if scheme == "" {
		scheme = "http"
	}
	addr := fmt.Sprintf("%s://%s:%d", scheme, sc.IpAddr, sc.Port)

	return &NamingClient{
		registryAddr: addr,
		client:       &http.Client{Timeout: 5 * time.Second},
		stopCh:       make(chan struct{}),
		subs:         make(map[string]chan struct{}),
	}, nil
}

func (c *NamingClient) RegisterInstance(param vo.RegisterInstanceParam) (bool, error) {
	ref := nacoscompat.ParseService(param.ServiceName, param.GroupName)
	metadata := nacoscompat.MetadataWithRuntime(param.Metadata, param.ClusterName, param.Ephemeral)
	body := map[string]interface{}{
		"id":           nacoscompat.BuildInstanceID(ref, param.ClusterName, param.Ip, int(param.Port)),
		"service_name": ref.FullName,
		"host":         param.Ip,
		"port":         param.Port,
		"weight":       int(param.Weight),
		"metadata":     metadata,
	}
	err := c.doPost("/v1/catalog/register", body)
	return err == nil, err
}

func (c *NamingClient) DeregisterInstance(param vo.DeregisterInstanceParam) (bool, error) {
	ref := nacoscompat.ParseService(param.ServiceName, param.GroupName)
	instanceID := nacoscompat.BuildInstanceID(ref, param.Cluster, param.Ip, int(param.Port))
	body := map[string]string{
		"service_name": ref.FullName,
		"instance_id":  instanceID,
		"status":       "offline",
	}
	err := c.doPost("/v1/catalog/instance/status", body)
	return err == nil, err
}

func (c *NamingClient) GetService(param vo.GetServiceParam) (nacosmodel.ServiceInfo, error) {
	instances, err := c.fetchInstances(param.ServiceName, param.GroupName, false)
	if err != nil {
		return nacosmodel.ServiceInfo{}, err
	}
	return nacosmodel.ServiceInfo{
		Name:  param.ServiceName,
		Hosts: instances,
	}, nil
}

func (c *NamingClient) SelectAllInstances(param vo.SelectAllInstancesParam) ([]nacosmodel.Instance, error) {
	return c.fetchInstances(param.ServiceName, param.GroupName, false)
}

func (c *NamingClient) SelectInstances(param vo.SelectInstancesParam) ([]nacosmodel.Instance, error) {
	return c.fetchInstances(param.ServiceName, param.GroupName, param.HealthyOnly)
}

func (c *NamingClient) SelectOneHealthyInstance(param vo.SelectOneHealthyInstanceParam) (*nacosmodel.Instance, error) {
	instances, err := c.fetchInstances(param.ServiceName, param.GroupName, true)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instance found for %s", param.ServiceName)
	}
	return &instances[0], nil
}

func (c *NamingClient) Subscribe(param *vo.SubscribeParam) error {
	ref := nacoscompat.ParseService(param.ServiceName, param.GroupName)
	stopCh := make(chan struct{})
	c.mu.Lock()
	c.subs[ref.FullName] = stopCh
	c.mu.Unlock()

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		var lastHash string
		for {
			select {
			case <-stopCh:
				return
			case <-c.stopCh:
				return
			case <-ticker.C:
				instances, err := c.fetchInstances(param.ServiceName, param.GroupName, false)
				if err != nil {
					param.SubscribeCallback(nil, err)
					continue
				}
				hash := instancesHash(instances)
				if hash != lastHash {
					lastHash = hash
					param.SubscribeCallback(instances, nil)
				}
			}
		}
	}()
	return nil
}

func (c *NamingClient) Unsubscribe(param *vo.SubscribeParam) error {
	ref := nacoscompat.ParseService(param.ServiceName, param.GroupName)
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.subs[ref.FullName]; ok {
		close(ch)
		delete(c.subs, ref.FullName)
	}
	return nil
}

func (c *NamingClient) GetAllServicesInfo(param vo.GetAllServiceInfoParam) (nacosmodel.ServiceList, error) {
	data, err := c.doGet("/v1/catalog/services")
	if err != nil {
		return nacosmodel.ServiceList{}, err
	}

	services, err := consulcompat.DecodeServicesMap(data)
	if err != nil {
		return nacosmodel.ServiceList{}, err
	}

	names := make([]string, 0, len(services))
	for name := range services {
		names = append(names, name)
	}

	return nacosmodel.ServiceList{
		Count: int64(len(names)),
		Doms:  names,
	}, nil
}

// --- internal helpers ---

func (c *NamingClient) fetchInstances(serviceName, groupName string, healthyOnly bool) ([]nacosmodel.Instance, error) {
	ref := nacoscompat.ParseService(serviceName, groupName)
	path := fmt.Sprintf("/v1/catalog/service/%s", ref.FullName)
	if healthyOnly {
		path += "?passing=true"
	}
	data, err := c.doGet(path)
	if err != nil {
		return nil, err
	}

	instances, err := consulcompat.DecodeCatalogInstances(data)
	if err != nil {
		return nil, err
	}

	result := make([]nacosmodel.Instance, 0, len(instances))
	for _, ei := range instances {
		serviceRef := nacoscompat.ParseService(ei.ServiceName, "")
		result = append(result, nacosmodel.Instance{
			InstanceId:  ei.ID,
			Ip:          ei.Address,
			Port:        uint64(ei.Port),
			Weight:      float64(ei.Weight),
			Healthy:     ei.Status == "passing",
			Enable:      true,
			Ephemeral:   true,
			ServiceName: serviceRef.Name,
			Metadata:    nacoscompat.UserMetadata(ei.Metadata),
		})
	}
	return result, nil
}

func (c *NamingClient) doPost(path string, body interface{}) error {
	data, _ := json.Marshal(body)
	resp, err := c.client.Post(c.registryAddr+path, "application/json", bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nacos-compat: status %d: %s", resp.StatusCode, string(b))
	}
	return nil
}

func (c *NamingClient) doGet(path string) ([]byte, error) {
	resp, err := c.client.Get(c.registryAddr + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nacos-compat: status %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func instancesHash(instances []nacosmodel.Instance) string {
	data, _ := json.Marshal(instances)
	return string(data)
}
