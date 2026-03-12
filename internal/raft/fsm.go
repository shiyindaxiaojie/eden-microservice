package raft

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	hraft "github.com/hashicorp/raft"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
)

// CommandType identifies the kind of Raft log entry.
type CommandType string

const (
	CmdRegister   CommandType = "register"
	CmdDeregister CommandType = "deregister"
	CmdHeartbeat  CommandType = "heartbeat"
)

// Command represents a Raft log command.
type Command struct {
	Type        CommandType     `json:"type"`
	Instance    *model.Instance `json:"instance,omitempty"`
	ServiceName string          `json:"service_name,omitempty"`
	InstanceID  string          `json:"instance_id,omitempty"`
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
		log.Printf("[FSM] failed to unmarshal command: %v", err)
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
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// snapshotData is the serializable form of the registry for Raft snapshots.
type snapshotData struct {
	Services map[string][]*model.Instance `json:"services"`
}

// Snapshot returns a snapshot of the FSM state.
func (f *FSM) Snapshot() (hraft.FSMSnapshot, error) {
	snap := f.registry.Snapshot()
	return &fsmSnapshot{data: snap}, nil
}

// Restore restores the FSM from a snapshot.
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()
	var sd snapshotData
	if err := json.NewDecoder(rc).Decode(&sd); err != nil {
		return err
	}
	f.registry.Restore(sd.Services)
	return nil
}

// fsmSnapshot implements raft.FSMSnapshot.
type fsmSnapshot struct {
	data map[string][]*model.Instance
}

func (s *fsmSnapshot) Persist(sink hraft.SnapshotSink) error {
	sd := snapshotData{Services: s.data}
	b, err := json.Marshal(sd)
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
