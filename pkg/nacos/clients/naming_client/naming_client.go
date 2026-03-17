// Package naming_client is a drop-in replacement for
// github.com/nacos-group/nacos-sdk-go/v2/clients/naming_client.
// It provides INamingClient interface backed by Eden Registry.
package naming_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	nacosmodel "github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/model"
	"github.com/shiyindaxiaojie/eden-go-registry/pkg/nacos/vo"
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

// NamingClient implements INamingClient by calling Eden Registry HTTP API.
type NamingClient struct {
	edenAddr string
	client   *http.Client
	stopCh   chan struct{}
	mu       sync.Mutex
	subs     map[string]chan struct{} // service -> stop channel
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
		edenAddr: addr,
		client:   &http.Client{Timeout: 5 * time.Second},
		stopCh:   make(chan struct{}),
		subs:     make(map[string]chan struct{}),
	}, nil
}

func (c *NamingClient) RegisterInstance(param vo.RegisterInstanceParam) (bool, error) {
	body := map[string]interface{}{
		"id":           fmt.Sprintf("%s-%s-%d", param.ServiceName, param.Ip, param.Port),
		"service_name": param.ServiceName,
		"host":         param.Ip,
		"port":         param.Port,
		"weight":       int(param.Weight),
		"metadata":     param.Metadata,
	}
	err := c.doPost("/v1/catalog/register", body)
	return err == nil, err
}

func (c *NamingClient) DeregisterInstance(param vo.DeregisterInstanceParam) (bool, error) {
	instanceID := fmt.Sprintf("%s-%s-%d", param.ServiceName, param.Ip, param.Port)
	body := map[string]string{
		"service_name": param.ServiceName,
		"instance_id":  instanceID,
	}
	err := c.doPost("/v1/catalog/deregister", body)
	return err == nil, err
}

func (c *NamingClient) GetService(param vo.GetServiceParam) (nacosmodel.ServiceInfo, error) {
	instances, err := c.fetchInstances(param.ServiceName, false)
	if err != nil {
		return nacosmodel.ServiceInfo{}, err
	}
	return nacosmodel.ServiceInfo{
		Name:  param.ServiceName,
		Hosts: instances,
	}, nil
}

func (c *NamingClient) SelectAllInstances(param vo.SelectAllInstancesParam) ([]nacosmodel.Instance, error) {
	return c.fetchInstances(param.ServiceName, false)
}

func (c *NamingClient) SelectInstances(param vo.SelectInstancesParam) ([]nacosmodel.Instance, error) {
	return c.fetchInstances(param.ServiceName, param.HealthyOnly)
}

func (c *NamingClient) SelectOneHealthyInstance(param vo.SelectOneHealthyInstanceParam) (*nacosmodel.Instance, error) {
	instances, err := c.fetchInstances(param.ServiceName, true)
	if err != nil {
		return nil, err
	}
	if len(instances) == 0 {
		return nil, fmt.Errorf("no healthy instance found for %s", param.ServiceName)
	}
	return &instances[0], nil
}

func (c *NamingClient) Subscribe(param *vo.SubscribeParam) error {
	stopCh := make(chan struct{})
	c.mu.Lock()
	c.subs[param.ServiceName] = stopCh
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
				instances, err := c.fetchInstances(param.ServiceName, false)
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if ch, ok := c.subs[param.ServiceName]; ok {
		close(ch)
		delete(c.subs, param.ServiceName)
	}
	return nil
}

func (c *NamingClient) GetAllServicesInfo(param vo.GetAllServiceInfoParam) (nacosmodel.ServiceList, error) {
	data, err := c.doGet("/v1/catalog/services")
	if err != nil {
		return nacosmodel.ServiceList{}, err
	}

	var services []struct {
		Name string `json:"name"`
	}
	json.Unmarshal(data, &services)

	names := make([]string, 0, len(services))
	for _, s := range services {
		names = append(names, s.Name)
	}

	return nacosmodel.ServiceList{
		Count: int64(len(names)),
		Doms:  names,
	}, nil
}

// --- internal helpers ---

func (c *NamingClient) fetchInstances(serviceName string, healthyOnly bool) ([]nacosmodel.Instance, error) {
	path := fmt.Sprintf("/v1/catalog/service/%s", serviceName)
	if healthyOnly {
		path += "?passing=true"
	}
	data, err := c.doGet(path)
	if err != nil {
		return nil, err
	}

	var edenInstances []struct {
		ID          string            `json:"id"`
		ServiceName string            `json:"service_name"`
		Host        string            `json:"host"`
		Port        int               `json:"port"`
		Weight      int               `json:"weight"`
		Status      string            `json:"status"`
		Metadata    map[string]string `json:"metadata"`
	}
	json.Unmarshal(data, &edenInstances)

	result := make([]nacosmodel.Instance, 0, len(edenInstances))
	for _, ei := range edenInstances {
		result = append(result, nacosmodel.Instance{
			InstanceId:  ei.ID,
			Ip:          ei.Host,
			Port:        uint64(ei.Port),
			Weight:      float64(ei.Weight),
			Healthy:     ei.Status == "passing",
			Enable:      true,
			Ephemeral:   true,
			ServiceName: ei.ServiceName,
			Metadata:    ei.Metadata,
		})
	}
	return result, nil
}

func (c *NamingClient) doPost(path string, body interface{}) error {
	data, _ := json.Marshal(body)
	resp, err := c.client.Post(c.edenAddr+path, "application/json", bytes.NewReader(data))
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
	resp, err := c.client.Get(c.edenAddr + path)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func instancesHash(instances []nacosmodel.Instance) string {
	data, _ := json.Marshal(instances)
	return string(data)
}
