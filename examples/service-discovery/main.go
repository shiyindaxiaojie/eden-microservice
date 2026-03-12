package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Instance model
type Instance struct {
	ID          string `json:"id"`
	ServiceName string `json:"service_name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Weight      int    `json:"weight"`
	Status      string `json:"status"`
}

const registryURL = "http://127.0.0.1:8500"

func main() {
	log.Println("=== Service Registration and Discovery Example ===")

	serviceName := "demo-service"
	instanceID := "demo-service-node-1"

	// 1. Register Service
	log.Printf("1. Registering instance '%s' for service '%s'...", instanceID, serviceName)
	inst := Instance{
		ID:          instanceID,
		ServiceName: serviceName,
		Host:        "192.168.1.100",
		Port:        8080,
		Weight:      100,
		Status:      "passing",
	}

	err := registerService(inst)
	if err != nil {
		log.Fatalf("Registration failed: %v", err)
	}
	log.Println("Registration successful!")

	// 2. Discover Service
	time.Sleep(1 * time.Second)
	log.Printf("\n2. Discovering service '%s'...", serviceName)
	instances, err := discoverService(serviceName)
	if err != nil {
		log.Fatalf("Discovery failed: %v", err)
	}
	
	log.Printf("Found %d healthy instances:", len(instances))
	for i, instance := range instances {
		log.Printf("  [%d] ID: %s, Address: %s:%d, Weight: %d", i+1, instance.ID, instance.Host, instance.Port, instance.Weight)
	}

	// 3. Heartbeat
	time.Sleep(1 * time.Second)
	log.Printf("\n3. Sending heartbeat for '%s'...", instanceID)
	err = heartbeatService(serviceName, instanceID)
	if err != nil {
		log.Printf("Heartbeat failed: %v", err)
	} else {
		log.Println("Heartbeat successful!")
	}

	// 4. Deregister Service
	time.Sleep(2 * time.Second)
	log.Printf("\n4. Deregistering instance '%s'...", instanceID)
	err = deregisterService(serviceName, instanceID)
	if err != nil {
		log.Fatalf("Deregistration failed: %v", err)
	}
	log.Println("Deregistration successful!")

	// 5. Verify Deregistration
	time.Sleep(1 * time.Second)
	log.Printf("\n5. Verifying deregistration...")
	instancesAfter, _ := discoverService(serviceName)
	log.Printf("Found %d instances for '%s' after deregistration.", len(instancesAfter), serviceName)

	log.Println("\n=== Example Completed successfully! ===")
}

func registerService(inst Instance) error {
	data, _ := json.Marshal(inst)
	resp, err := http.Post(registryURL+"/v1/catalog/register", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func discoverService(serviceName string) ([]Instance, error) {
	resp, err := http.Get(fmt.Sprintf("%s/v1/catalog/service/%s?passing=true", registryURL, serviceName))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
	}

	var instances []Instance
	if err := json.NewDecoder(resp.Body).Decode(&instances); err != nil {
		return nil, err
	}
	return instances, nil
}

func heartbeatService(serviceName, instanceID string) error {
	reqBody := map[string]string{
		"service_name": serviceName,
		"instance_id":  instanceID,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := http.Post(registryURL+"/v1/catalog/heartbeat", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}

func deregisterService(serviceName, instanceID string) error {
	reqBody := map[string]string{
		"service_name": serviceName,
		"instance_id":  instanceID,
	}
	data, _ := json.Marshal(reqBody)
	resp, err := http.Post(registryURL+"/v1/catalog/deregister", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
	}
	return nil
}
