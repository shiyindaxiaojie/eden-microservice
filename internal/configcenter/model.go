package configcenter

import (
	"errors"
	"strings"
	"time"
)

const (
	DefaultNamespace = "default"
	DefaultGroup     = "DEFAULT_GROUP"

	HistoryPublish = "publish"
	HistoryDelete  = "delete"
)

var (
	ErrNotFound        = errors.New("config not found")
	ErrConflict        = errors.New("config md5 conflict")
	ErrInvalidIdentity = errors.New("invalid config identity")
	ErrTooManyTargets  = errors.New("too many config watch targets")
	ErrTooManyWaiters  = errors.New("too many config waiters")
)

type Identity struct {
	Namespace string `json:"namespace"`
	Group     string `json:"group"`
	DataID    string `json:"data_id"`
}

func NormalizeIdentity(identity Identity) (Identity, error) {
	identity.Namespace = strings.TrimSpace(identity.Namespace)
	identity.Group = strings.TrimSpace(identity.Group)
	identity.DataID = strings.TrimSpace(identity.DataID)
	if identity.Namespace == "" {
		identity.Namespace = DefaultNamespace
	}
	if identity.Group == "" {
		identity.Group = DefaultGroup
	}
	if identity.DataID == "" || identity.DataID == "." || identity.DataID == ".." || strings.ContainsAny(identity.DataID, `/\`) {
		return Identity{}, ErrInvalidIdentity
	}
	return identity, nil
}

type Resource struct {
	Identity
	Content     string    `json:"content"`
	Type        string    `json:"type"`
	MD5         string    `json:"md5"`
	Revision    uint64    `json:"revision"`
	Description string    `json:"description"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	CreatedBy   string    `json:"created_by"`
	UpdatedBy   string    `json:"updated_by"`
}

type PublishRequest struct {
	Identity
	Content     string   `json:"content"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	ExpectedMD5 string   `json:"expected_md5"`
	Operator    string   `json:"-"`
}

type HistoryEntry struct {
	Identity
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	MD5       string    `json:"md5"`
	Revision  uint64    `json:"revision"`
	Action    string    `json:"action"`
	Operator  string    `json:"operator"`
	Summary   string    `json:"summary"`
	CreatedAt time.Time `json:"created_at"`
}

type ListQuery struct {
	Namespace string
	Group     string
	Type      string
	Query     string
	Page      int
	PageSize  int
}

type ListResult struct {
	Total int        `json:"total"`
	Data  []Resource `json:"data"`
}

type WatchTarget struct {
	Identity
	MD5 string `json:"md5"`
}

type Change struct {
	Identity
	MD5      string `json:"md5"`
	Revision uint64 `json:"revision"`
}
