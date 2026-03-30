package nacos

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"sync"
	"time"

	nacosgrpc "github.com/nacos-group/nacos-sdk-go/v2/api/grpc"
	nacosconstant "github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	rpcrequest "github.com/nacos-group/nacos-sdk-go/v2/common/remote/rpc/rpc_request"
	rpcresponse "github.com/nacos-group/nacos-sdk-go/v2/common/remote/rpc/rpc_response"
	nacosmodel "github.com/nacos-group/nacos-sdk-go/v2/model"
	nacoscompat "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos/compat"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"google.golang.org/grpc/peer"
	"google.golang.org/protobuf/types/known/anypb"
)

type NacosNamingServer struct {
	nacosgrpc.UnimplementedRequestServer
	nacosgrpc.UnimplementedBiRequestStreamServer

	config  *config.Config
	catalog catalog.Registry

	mu          sync.Mutex
	connections map[string]*nacosConnection
}

type nacosConnection struct {
	key    string
	stream nacosgrpc.BiRequestStream_RequestBiStreamServer

	sendMu sync.Mutex
	done   chan struct{}

	subscriptions map[string]*nacosSubscription
}

type nacosSubscription struct {
	key        string
	namespace  string
	ref        nacoscompat.ServiceRef
	storedName string
	clusters   string
	ch         chan catalog.WatchEvent
}

func NewNacosNamingServer(cfg *config.Config, catalogRegistry catalog.Registry) *NacosNamingServer {
	return &NacosNamingServer{
		config:      cfg,
		catalog:     catalogRegistry,
		connections: make(map[string]*nacosConnection),
	}
}

func (s *NacosNamingServer) Request(ctx context.Context, payload *nacosgrpc.Payload) (*nacosgrpc.Payload, error) {
	requestType := payload.GetMetadata().GetType()
	requestID := ""

	switch requestType {
	case "ServerCheckRequest":
		req := rpcrequest.NewServerCheckRequest()
		requestID = decodeRequest(payload, req)
		resp := &rpcresponse.ServerCheckResponse{
			Response: &rpcresponse.Response{
				ResultCode: nacosconstant.RESPONSE_CODE_SUCCESS,
				Success:    true,
				RequestId:  requestID,
			},
			ConnectionId: connectionKeyFromContext(ctx),
		}
		return marshalPayload(resp.GetResponseType(), resp)
	case "HealthCheckRequest":
		req := rpcrequest.NewHealthCheckRequest()
		requestID = decodeRequest(payload, req)
		resp := &rpcresponse.HealthCheckResponse{
			Response: &rpcresponse.Response{
				ResultCode: nacosconstant.RESPONSE_CODE_SUCCESS,
				Success:    true,
				RequestId:  requestID,
			},
		}
		return marshalPayload(resp.GetResponseType(), resp)
	case nacosconstant.INSTANCE_REQUEST_NAME:
		req := &rpcrequest.InstanceRequest{NamingRequest: rpcrequest.NewNamingRequest("", "", "")}
		requestID = decodeRequest(payload, req)
		resp := s.handleInstanceRequest(req, requestID)
		return marshalResponse(resp)
	case nacosconstant.BATCH_INSTANCE_REQUEST_NAME:
		req := &rpcrequest.BatchInstanceRequest{NamingRequest: rpcrequest.NewNamingRequest("", "", "")}
		requestID = decodeRequest(payload, req)
		resp := s.handleBatchInstanceRequest(req, requestID)
		return marshalResponse(resp)
	case nacosconstant.SERVICE_LIST_REQUEST_NAME:
		req := &rpcrequest.ServiceListRequest{NamingRequest: rpcrequest.NewNamingRequest("", "", "")}
		requestID = decodeRequest(payload, req)
		resp := s.handleServiceListRequest(req, requestID)
		return marshalResponse(resp)
	case nacosconstant.SUBSCRIBE_SERVICE_REQUEST_NAME:
		req := &rpcrequest.SubscribeServiceRequest{NamingRequest: rpcrequest.NewNamingRequest("", "", "")}
		requestID = decodeRequest(payload, req)
		resp := s.handleSubscribeServiceRequest(ctx, req, requestID)
		return marshalResponse(resp)
	case nacosconstant.SERVICE_QUERY_REQUEST_NAME:
		req := &rpcrequest.ServiceQueryRequest{NamingRequest: rpcrequest.NewNamingRequest("", "", "")}
		requestID = decodeRequest(payload, req)
		resp := s.handleServiceQueryRequest(req, requestID)
		return marshalResponse(resp)
	default:
		resp := &rpcresponse.ErrorResponse{
			Response: &rpcresponse.Response{
				ResultCode: 500,
				ErrorCode:  500,
				Success:    false,
				Message:    "unsupported nacos request: " + requestType,
				RequestId:  requestID,
			},
		}
		return marshalPayload(resp.GetResponseType(), resp)
	}
}

func (s *NacosNamingServer) RequestBiStream(stream nacosgrpc.BiRequestStream_RequestBiStreamServer) error {
	key := connectionKeyFromContext(stream.Context())
	conn := &nacosConnection{
		key:           key,
		stream:        stream,
		done:          make(chan struct{}),
		subscriptions: make(map[string]*nacosSubscription),
	}

	s.mu.Lock()
	if old := s.connections[key]; old != nil {
		s.cleanupConnectionLocked(old)
	}
	s.connections[key] = conn
	s.mu.Unlock()

	defer s.cleanupConnection(conn)

	for {
		payload, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		switch payload.GetMetadata().GetType() {
		case "ConnectionSetupRequest", "NotifySubscriberResponse":
			continue
		default:
			continue
		}
	}
}

func (s *NacosNamingServer) handleInstanceRequest(req *rpcrequest.InstanceRequest, requestID string) rpcresponse.IResponse {
	ref := nacoscompat.ParseService(req.ServiceName, req.GroupName)
	namespace := nacoscompat.NormalizeNamespace(req.Namespace)
	clusterName := req.Instance.ClusterName
	instanceID := nacoscompat.BuildInstanceID(ref, clusterName, req.Instance.Ip, int(req.Instance.Port))

	switch req.Type {
	case "registerInstance":
		inst := &catalog.Instance{
			ID:          instanceID,
			ServiceName: ref.FullName,
			Namespace:   namespace,
			Host:        req.Instance.Ip,
			Port:        int(req.Instance.Port),
			Weight:      weightFromFloat(req.Instance.Weight),
			Metadata:    nacoscompat.MetadataWithRuntime(req.Instance.Metadata, clusterName, req.Instance.Ephemeral),
		}
		if err := s.catalog.Register(inst); err != nil {
			return errorResponse(requestID, err)
		}
		if !req.Instance.Healthy || !req.Instance.Enable {
			if err := s.catalog.SetInstanceStatus(namespace, inst.ServiceName, inst.ID, "offline"); err != nil {
				return errorResponse(requestID, err)
			}
		}
	case "deregisterInstance":
		inst, err := s.findNacosInstance(namespace, ref, clusterName, req.Instance.Ip, int(req.Instance.Port))
		if err == nil {
			if err := s.catalog.Deregister(namespace, inst.ServiceName, inst.ID); err != nil {
				return errorResponse(requestID, err)
			}
		}
	default:
		return errorResponse(requestID, fmt.Errorf("unsupported instance request type: %s", req.Type))
	}

	return &rpcresponse.InstanceResponse{
		Response: &rpcresponse.Response{
			ResultCode: nacosconstant.RESPONSE_CODE_SUCCESS,
			Success:    true,
			RequestId:  requestID,
		},
	}
}

func (s *NacosNamingServer) handleBatchInstanceRequest(req *rpcrequest.BatchInstanceRequest, requestID string) rpcresponse.IResponse {
	for _, instance := range req.Instances {
		single := &rpcrequest.InstanceRequest{
			NamingRequest: rpcrequest.NewNamingRequest(req.Namespace, req.ServiceName, req.GroupName),
			Type:          req.Type,
			Instance:      instance,
		}
		if resp := s.handleInstanceRequest(single, requestID); !resp.IsSuccess() {
			return resp
		}
	}

	return &rpcresponse.BatchInstanceResponse{
		Response: &rpcresponse.Response{
			ResultCode: nacosconstant.RESPONSE_CODE_SUCCESS,
			Success:    true,
			RequestId:  requestID,
		},
	}
}

func (s *NacosNamingServer) handleServiceListRequest(req *rpcrequest.ServiceListRequest, requestID string) rpcresponse.IResponse {
	namespace := nacoscompat.NormalizeNamespace(req.Namespace)
	names, err := s.serviceNames(namespace)
	if err != nil {
		return errorResponse(requestID, err)
	}

	filtered := make([]string, 0, len(names))
	for _, name := range names {
		ref := nacoscompat.ParseService(name, "")
		if req.GroupName != "" && ref.GroupName != req.GroupName {
			continue
		}
		filtered = append(filtered, name)
	}

	pageNo := req.PageNo
	if pageNo <= 0 {
		pageNo = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 10
	}

	list := nacoscompat.ToServiceList(filtered, pageNo, pageSize)
	return &rpcresponse.ServiceListResponse{
		Response: &rpcresponse.Response{
			ResultCode: nacosconstant.RESPONSE_CODE_SUCCESS,
			Success:    true,
			RequestId:  requestID,
		},
		Count:        int(list.Count),
		ServiceNames: list.Doms,
	}
}

func (s *NacosNamingServer) handleSubscribeServiceRequest(ctx context.Context, req *rpcrequest.SubscribeServiceRequest, requestID string) rpcresponse.IResponse {
	ref := nacoscompat.ParseService(req.ServiceName, req.GroupName)
	namespace := nacoscompat.NormalizeNamespace(req.Namespace)
	instances, storedName, err := s.nacosServiceInstances(namespace, ref, false)
	if err != nil {
		return errorResponse(requestID, err)
	}

	if conn := s.connectionFromContext(ctx); conn != nil {
		if req.Subscribe {
			s.addSubscription(conn, namespace, ref, storedName, req.Clusters)
		} else {
			s.removeSubscription(conn, namespace, storedName, req.Clusters)
		}
	}

	service := buildNacosService(ref, req.Clusters, instances)
	return &rpcresponse.SubscribeServiceResponse{
		Response: &rpcresponse.Response{
			ResultCode: nacosconstant.RESPONSE_CODE_SUCCESS,
			Success:    true,
			RequestId:  requestID,
		},
		ServiceInfo: service,
	}
}

func (s *NacosNamingServer) handleServiceQueryRequest(req *rpcrequest.ServiceQueryRequest, requestID string) rpcresponse.IResponse {
	ref := nacoscompat.ParseService(req.ServiceName, req.GroupName)
	namespace := nacoscompat.NormalizeNamespace(req.Namespace)
	instances, _, err := s.nacosServiceInstances(namespace, ref, req.HealthyOnly)
	if err != nil {
		return errorResponse(requestID, err)
	}

	service := buildNacosService(ref, req.Cluster, instances)
	return &rpcresponse.QueryServiceResponse{
		Response: &rpcresponse.Response{
			ResultCode: nacosconstant.RESPONSE_CODE_SUCCESS,
			Success:    true,
			RequestId:  requestID,
		},
		ServiceInfo: service,
	}
}

func (s *NacosNamingServer) connectionFromContext(ctx context.Context) *nacosConnection {
	key := connectionKeyFromContext(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connections[key]
}

func (s *NacosNamingServer) addSubscription(conn *nacosConnection, namespace string, ref nacoscompat.ServiceRef, storedName, clusters string) {
	key := subscriptionKey(namespace, storedName, clusters)

	s.mu.Lock()
	if _, ok := conn.subscriptions[key]; ok {
		s.mu.Unlock()
		return
	}

	ch := make(chan catalog.WatchEvent, 8)
	sub := &nacosSubscription{
		key:        key,
		namespace:  namespace,
		ref:        ref,
		storedName: storedName,
		clusters:   clusters,
		ch:         ch,
	}
	conn.subscriptions[key] = sub
	s.mu.Unlock()

	s.catalog.Subscribe(namespace, storedName, "", ch)
	go s.forwardSubscription(conn, sub)
}

func (s *NacosNamingServer) removeSubscription(conn *nacosConnection, namespace, storedName, clusters string) {
	key := subscriptionKey(namespace, storedName, clusters)

	s.mu.Lock()
	sub, ok := conn.subscriptions[key]
	if ok {
		delete(conn.subscriptions, key)
	}
	s.mu.Unlock()

	if ok {
		s.catalog.Unsubscribe(sub.namespace, sub.storedName, sub.ch)
		close(sub.ch)
	}
}

func (s *NacosNamingServer) forwardSubscription(conn *nacosConnection, sub *nacosSubscription) {
	for {
		select {
		case <-conn.done:
			return
		case evt, ok := <-sub.ch:
			if !ok {
				return
			}

			service := buildNacosService(sub.ref, sub.clusters, evt.Instances)
			req := &rpcrequest.NotifySubscriberRequest{
				NamingRequest: rpcrequest.NewNamingRequest(sub.namespace, sub.ref.Name, sub.ref.GroupName),
				ServiceInfo:   service,
			}
			req.RequestId = fmt.Sprintf("notify-%d", time.Now().UnixNano())

			payload, err := marshalPayload(req.GetRequestType(), req)
			if err != nil {
				return
			}
			if err := conn.send(payload); err != nil {
				return
			}
		}
	}
}

func (s *NacosNamingServer) cleanupConnection(conn *nacosConnection) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupConnectionLocked(conn)
}

func (s *NacosNamingServer) cleanupConnectionLocked(conn *nacosConnection) {
	current, ok := s.connections[conn.key]
	if !ok || current != conn {
		return
	}

	delete(s.connections, conn.key)
	select {
	case <-conn.done:
	default:
		close(conn.done)
	}
	for _, sub := range conn.subscriptions {
		s.catalog.Unsubscribe(sub.namespace, sub.storedName, sub.ch)
		close(sub.ch)
	}
	conn.subscriptions = map[string]*nacosSubscription{}
}

func (c *nacosConnection) send(payload *nacosgrpc.Payload) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	return c.stream.Send(payload)
}

func (s *NacosNamingServer) nacosServiceInstances(namespace string, ref nacoscompat.ServiceRef, healthyOnly bool) ([]*catalog.Instance, string, error) {
	for _, candidate := range nacoscompat.CandidateStoredNames(ref) {
		instances, err := s.catalog.GetService(namespace, candidate, healthyOnly)
		if err != nil {
			return nil, "", err
		}
		if len(instances) > 0 {
			return instances, candidate, nil
		}
	}
	return []*catalog.Instance{}, ref.FullName, nil
}

func (s *NacosNamingServer) findNacosInstance(namespace string, ref nacoscompat.ServiceRef, clusterName, address string, port int) (*catalog.Instance, error) {
	instances, _, err := s.nacosServiceInstances(namespace, ref, false)
	if err != nil {
		return nil, err
	}

	expectedID := nacoscompat.BuildInstanceID(ref, clusterName, address, port)
	targetCluster := clusterName
	if targetCluster == "" {
		targetCluster = nacoscompat.DefaultCluster
	}

	for _, inst := range instances {
		if inst.ID == expectedID {
			return inst, nil
		}
		if inst.Host == address && inst.Port == port && nacoscompat.ClusterName(inst.Metadata) == targetCluster {
			return inst, nil
		}
	}
	return nil, fmt.Errorf("instance not found")
}

func (s *NacosNamingServer) serviceNames(namespace string) ([]string, error) {
	services, err := s.catalog.ListServices(namespace)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(services))
	for _, item := range services {
		service, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := service["name"].(string)
		if name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func buildNacosService(ref nacoscompat.ServiceRef, clusters string, instances []*catalog.Instance) nacosmodel.Service {
	payload := make([]nacoscompat.ServiceInstance, 0, len(instances))
	for _, inst := range instances {
		payload = append(payload, nacoscompat.ServiceInstance{
			ID:          inst.ID,
			ServiceName: inst.ServiceName,
			Address:     inst.Host,
			Port:        inst.Port,
			Weight:      inst.Weight,
			Metadata:    inst.Metadata,
			Healthy:     inst.Status == catalog.HealthPassing,
		})
	}
	return nacoscompat.ToModelService(ref, clusters, payload)
}

func decodeRequest(payload *nacosgrpc.Payload, request rpcrequest.IRequest) string {
	if body := payload.GetBody(); body != nil && len(body.Value) > 0 {
		_ = json.Unmarshal(body.Value, request)
	}
	return request.GetRequestId()
}

func marshalResponse(response rpcresponse.IResponse) (*nacosgrpc.Payload, error) {
	return marshalPayload(response.GetResponseType(), response)
}

func marshalPayload(payloadType string, body interface{}) (*nacosgrpc.Payload, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	return &nacosgrpc.Payload{
		Metadata: &nacosgrpc.Metadata{
			Type: payloadType,
		},
		Body: &anypb.Any{Value: raw},
	}, nil
}

func errorResponse(requestID string, err error) rpcresponse.IResponse {
	return &rpcresponse.ErrorResponse{
		Response: &rpcresponse.Response{
			ResultCode: 500,
			ErrorCode:  500,
			Success:    false,
			Message:    err.Error(),
			RequestId:  requestID,
		},
	}
}

func connectionKeyFromContext(ctx context.Context) string {
	if p, ok := peer.FromContext(ctx); ok && p.Addr != nil {
		return p.Addr.String()
	}
	return fmt.Sprintf("nacos-%d", time.Now().UnixNano())
}

func subscriptionKey(namespace, storedName, clusters string) string {
	return namespace + "\x00" + storedName + "\x00" + clusters
}

func weightFromFloat(value float64) int {
	if value <= 0 {
		return 1
	}
	rounded := int(value + 0.5)
	if rounded <= 0 {
		return 1
	}
	return rounded
}
