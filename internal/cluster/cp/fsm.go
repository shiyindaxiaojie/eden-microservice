package cp

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/shiyindaxiaojie/eden-go-logger"
	hraft "github.com/hashicorp/raft"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// CommandType identifies the kind of Raft log entry.
type CommandType string

const (
	CmdRegister      CommandType = "register"
	CmdDeregister    CommandType = "deregister"
	CmdHeartbeat     CommandType = "heartbeat"
	CmdAddAPIKey     CommandType = "add_api_key"
	CmdDeleteAPIKey  CommandType = "delete_api_key"
	CmdAddUser       CommandType = "add_user"
	CmdDeleteUser    CommandType = "delete_user"
	CmdSetMode       CommandType = "set_mode"
	CmdSetEnv        CommandType = "set_env"
	CmdSetSeeds      CommandType = "set_seeds"
)

// Command represents a Raft log command.
type Command struct {
	Type        CommandType     `json:"type"`
	Instance    *model.Instance `json:"instance,omitempty"`
	ServiceName string          `json:"service_name,omitempty"`
	InstanceID  string          `json:"instance_id,omitempty"`
	APIKey      *model.APIKey   `json:"api_key,omitempty"`
	User        *model.User     `json:"user,omitempty"`
	Key         string          `json:"key,omitempty"`      // for delete operations
	Username    string          `json:"username,omitempty"` // for delete operations
	Mode        string          `json:"mode,omitempty"`     // for set_mode
	Environment string          `json:"environment,omitempty"` // for set_env
	Seeds       []string        `json:"seeds,omitempty"`       // for set_seeds
}

// FSM implements hashicorp/raft.FSM backed by an in-memory Registry.
type FSM struct {
	registry *store.Registry
}

// NewFSM creates a new FSM wrapping the registry.
func NewFSM(registry *store.Registry) *FSM {
	return &FSM{registry: registry}
}

// Apply is called by Raft once a log entry is committed.
func (f *FSM) Apply(l *hraft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(l.Data, &cmd); err != nil {
		logger.Error("[FSM] failed to unmarshal command: %v", err)
		return err
	}

	switch cmd.Type {
	case CmdRegister:
		f.registry.Register(cmd.Instance)
		return nil
	case CmdDeregister:
		ok := f.registry.Deregister(cmd.ServiceName, cmd.InstanceID)
		return ok
	case CmdHeartbeat:
		ok := f.registry.Heartbeat(cmd.ServiceName, cmd.InstanceID)
		return ok
	case CmdAddAPIKey:
		f.registry.AddAPIKey(cmd.APIKey)
		return nil
	case CmdDeleteAPIKey:
		f.registry.DeleteAPIKey(cmd.Key)
		return nil
	case CmdAddUser:
		f.registry.AddUser(cmd.User)
		return nil
	case CmdDeleteUser:
		f.registry.DeleteUser(cmd.Username)
		return nil
	case CmdSetMode:
		f.registry.SetMode(cmd.Mode)
		return nil
	case CmdSetEnv:
		f.registry.SetEnvironment(cmd.Environment)
		return nil
	case CmdSetSeeds:
		f.registry.SetSeeds(cmd.Seeds)
		return nil
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// Snapshot returns a snapshot of the FSM state.
func (f *FSM) Snapshot() (hraft.FSMSnapshot, error) {
	snap := f.registry.Snapshot()
	return &fsmSnapshot{data: snap}, nil
}

// Restore restores the FSM from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var sd store.SnapshotData
	if err := json.NewDecoder(rc).Decode(&sd); err != nil {
		return err
	}
	f.registry.Restore(&sd)
	return nil
}

// fsmSnapshot implements raft.FSMSnapshot.
type fsmSnapshot struct {
	data *store.SnapshotData
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
