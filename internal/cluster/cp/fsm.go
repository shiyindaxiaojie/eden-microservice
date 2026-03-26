package cp

import (
	"encoding/json"
	"fmt"
	"io"

	hraft "github.com/hashicorp/raft"
	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	clustercmd "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/command"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/state"
)

// FSM implements hashicorp/raft.FSM backed by an in-memory Registry.
type FSM struct {
	state *state.State
}

// NewFSM creates a new FSM wrapping the registry.
func NewFSM(runtimeState *state.State) *FSM {
	return &FSM{state: runtimeState}
}

// Apply is called by Raft once a log entry is committed.
func (f *FSM) Apply(l *hraft.Log) interface{} {
	var cmd clustercmd.Command
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		logger.Error("[FSM] failed to unmarshal command: %v", err)
		return err
	}

	switch cmd.Type {
	case clustercmd.CmdRegister:
		f.state.Register(fromReplicatedInstance(cmd.Instance))
		return nil
	case clustercmd.CmdDeregister:
		_, ok := f.state.Catalog.Instances.DeregisterNS(cmd.Namespace, cmd.ServiceName, cmd.InstanceID)
		return ok
	case clustercmd.CmdHeartbeat:
		inst, _ := f.state.HeartbeatNS(cmd.Namespace, cmd.ServiceName, cmd.InstanceID)
		if inst == nil {
			return fmt.Errorf("instance not found")
		}
		return nil
	case clustercmd.CmdAddAPIKey:
		f.state.AddAPIKey(fromReplicatedAPIKey(cmd.APIKey))
		return nil
	case clustercmd.CmdDeleteAPIKey:
		f.state.DeleteAPIKey(cmd.Key)
		return nil
	case clustercmd.CmdAddUser:
		f.state.AddUser(fromReplicatedUser(cmd.User))
		return nil
	case clustercmd.CmdDeleteUser:
		f.state.DeleteUser(cmd.Username)
		return nil
	case clustercmd.CmdSetMode:
		f.state.SetMode(cmd.Mode)
		return nil
	case clustercmd.CmdSetEnv:
		f.state.SetEnvironment(cmd.Environment)
		return nil
	case clustercmd.CmdSetSeeds:
		f.state.SetSeeds(cmd.Seeds)
		return nil
	case clustercmd.CmdSetLogLevel:
		f.state.SetLogLevel(cmd.LogLevel)
		if lg, ok := logger.GetLogger().(*logger.Logger); ok {
			lg.SetLevel(logger.ParseLevel(cmd.LogLevel))
		}
		return nil
	case clustercmd.CmdSetEventRetentionDays:
		f.state.SetEventRetentionDays(cmd.IntValue)
		return nil
	case clustercmd.CmdSetLogRetentionDays:
		f.state.SetLogRetentionDays(cmd.IntValue)
		return nil
	case clustercmd.CmdSetEventTypes:
		f.state.SetEventTypes(cmd.StringList)
		return nil
	case clustercmd.CmdSetHeartbeatMaxFailures:
		f.state.SetHeartbeatMaxFailures(cmd.IntValue)
		return nil
	case clustercmd.CmdSetInstanceRemovalDelaySeconds:
		f.state.SetInstanceRemovalDelaySeconds(cmd.IntValue)
		return nil
	case clustercmd.CmdSetAPIKeyAuthEnabled:
		f.state.SetAPIKeyAuthEnabled(cmd.BoolValue)
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
	var sd state.SnapshotData
	if err := json.NewDecoder(rc).Decode(&sd); err != nil {
		return err
	}
	f.state.Restore(&sd)
	return nil
}

// fsmSnapshot implements raft.FSMSnapshot.
type fsmSnapshot struct {
	data *state.SnapshotData
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

func fromReplicatedInstance(inst *clustercmd.Instance) *catalog.Instance {
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

func fromReplicatedUser(user *clustercmd.User) *auth.User {
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

func fromReplicatedAPIKey(key *clustercmd.APIKey) *auth.APIKey {
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
