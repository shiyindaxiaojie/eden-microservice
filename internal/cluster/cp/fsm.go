package cp

import (
	"encoding/json"
	"fmt"
	"io"

	hraft "github.com/hashicorp/raft"
	logger "github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	clusterpkg "github.com/shiyindaxiaojie/eden-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-registry/internal/cluster/replication"
)

// FSM implements hashicorp/raft.FSM backed by an in-memory Registry.
type FSM struct {
	state *clusterpkg.RuntimeState
}

// NewFSM creates a new FSM wrapping the registry.
func NewFSM(runtimeState *clusterpkg.RuntimeState) *FSM {
	return &FSM{state: runtimeState}
}

// Apply is called by Raft once a log entry is committed.
func (f *FSM) Apply(l *hraft.Log) interface{} {
	var cmd replication.Command
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		logger.Error("[FSM] failed to unmarshal command: %v", err)
		return err
	}

	switch cmd.Type {
	case replication.CmdRegister:
		f.state.Register(fromReplicatedInstance(cmd.Instance))
		return nil
	case replication.CmdDeregister:
		_, ok := f.state.Catalog.Instances.DeregisterNS(cmd.Namespace, cmd.ServiceName, cmd.InstanceID)
		return ok
	case replication.CmdSetInstanceStatus:
		var healthStatus catalog.HealthStatus
		switch cmd.Status {
		case "online":
			healthStatus = catalog.HealthPassing
		case "offline":
			healthStatus = catalog.HealthCritical
		default:
			return fmt.Errorf("invalid status: %s", cmd.Status)
		}
		inst, ok := f.state.SetInstanceStatus(cmd.Namespace, cmd.ServiceName, cmd.InstanceID, healthStatus)
		if !ok || inst == nil {
			return fmt.Errorf("instance not found")
		}
		return nil
	case replication.CmdHeartbeat:
		inst, _ := f.state.HeartbeatNS(cmd.Namespace, cmd.ServiceName, cmd.InstanceID)
		if inst == nil {
			return fmt.Errorf("instance not found")
		}
		return nil
	case replication.CmdAddAPIKey:
		f.state.AddAPIKey(fromReplicatedAPIKey(cmd.APIKey))
		return nil
	case replication.CmdDeleteAPIKey:
		f.state.DeleteAPIKey(cmd.Key)
		return nil
	case replication.CmdAddUser:
		f.state.AddUser(fromReplicatedUser(cmd.User))
		return nil
	case replication.CmdDeleteUser:
		f.state.DeleteUser(cmd.Username)
		return nil
	case replication.CmdSetMode:
		f.state.SetMode(cmd.Mode)
		return nil
	case replication.CmdSetEnv:
		f.state.SetEnvironment(cmd.Environment)
		return nil
	case replication.CmdSetSeeds:
		f.state.SetSeeds(cmd.Seeds)
		return nil
	case replication.CmdSetLogLevel:
		f.state.SetLogLevel(cmd.LogLevel)
		if lg, ok := logger.GetLogger().(*logger.Logger); ok {
			lg.SetLevel(logger.ParseLevel(cmd.LogLevel))
		}
		return nil
	case replication.CmdSetEventRetentionDays:
		f.state.SetEventRetentionDays(cmd.IntValue)
		return nil
	case replication.CmdSetLogRetentionDays:
		f.state.SetLogRetentionDays(cmd.IntValue)
		return nil
	case replication.CmdSetEventTypes:
		f.state.SetEventTypes(cmd.StringList)
		return nil
	case replication.CmdSetHeartbeatMaxFailures:
		f.state.SetHeartbeatMaxFailures(cmd.IntValue)
		return nil
	case replication.CmdSetInstanceRemovalDelaySeconds:
		f.state.SetInstanceRemovalDelaySeconds(cmd.IntValue)
		return nil
	case replication.CmdSetAPIKeyAuthEnabled:
		f.state.SetAPIKeyAuthEnabled(cmd.BoolValue)
		return nil
	case replication.CmdSetNotifyAlertNodeID:
		f.state.SetNotifyAlertNodeID(cmd.NodeID)
		return nil
	case replication.CmdSetRegistryFlushMode:
		f.state.SetRegistryFlushMode(cmd.StringValue)
		return nil
	case replication.CmdSetRegistryFlushIntervalMS:
		f.state.SetRegistryFlushIntervalMS(cmd.IntValue)
		return nil
	case replication.CmdSetEventStorageMode:
		f.state.SetEventStorageMode(cmd.StringValue)
		return nil
	case replication.CmdSetMetricsStorageMode:
		f.state.SetMetricsStorageMode(cmd.StringValue)
		return nil
	case replication.CmdSetMetricsRetentionDays:
		f.state.SetMetricsRetentionDays(cmd.IntValue)
		return nil
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// Snapshot returns a snapshot of the FSM state.
func (f *FSM) Snapshot() (hraft.FSMSnapshot, error) {
	snap := f.state.Snapshot()
	return &fsmSnapshot{data: snap}, nil
}

// Restore restores the FSM from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var sd clusterpkg.SnapshotData
	if err := json.NewDecoder(rc).Decode(&sd); err != nil {
		return err
	}
	f.state.Restore(&sd)
	return nil
}

// fsmSnapshot implements raft.FSMSnapshot.
type fsmSnapshot struct {
	data *clusterpkg.SnapshotData
}

func (s *fsmSnapshot) Persist(sink hraft.SnapshotSink) error {
	b, err := json.Marshal(s.data)
	if err != nil {
		sink.Cancel()
		return err
	}
	if _, err := sink.Write(b); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}

func fromReplicatedInstance(inst *replication.Instance) *catalog.Instance {
	if inst == nil {
		return nil
	}
	return &catalog.Instance{
		ID:            inst.ID,
		ServiceName:   inst.ServiceName,
		Namespace:     inst.Namespace,
		Host:          inst.Host,
		Port:          inst.Port,
		Weight:        inst.Weight,
		Datacenter:    inst.Datacenter,
		Metadata:      inst.Metadata,
		Status:        catalog.HealthStatus(inst.Status),
		ManualOffline: inst.ManualOffline,
		LastHeartbeat: inst.LastHeartbeat,
		RegisteredAt:  inst.RegisteredAt,
	}
}

func fromReplicatedUser(user *replication.User) *auth.User {
	if user == nil {
		return nil
	}
	return &auth.User{
		Username:  user.Username,
		Password:  user.Password,
		Nickname:  user.Nickname,
		Phone:     user.Phone,
		Email:     user.Email,
		Remark:    user.Remark,
		Role:      user.Role,
		IsBuiltIn: user.IsBuiltIn,
	}
}

func fromReplicatedAPIKey(key *replication.APIKey) *auth.APIKey {
	if key == nil {
		return nil
	}
	return &auth.APIKey{
		Key:         key.Key,
		Label:       key.Label,
		Description: key.Description,
		CreatedBy:   key.CreatedBy,
		CreatedAt:   key.CreatedAt,
		ExpiresAt:   key.ExpiresAt,
		Status:      key.Status,
	}
}
