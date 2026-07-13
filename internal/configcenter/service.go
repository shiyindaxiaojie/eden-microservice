package configcenter

import "time"

type Service interface {
	Get(Identity) (*Resource, error)
	List(ListQuery) (ListResult, error)
	Publish(PublishRequest) (*Resource, error)
	Delete(Identity, string) (*HistoryEntry, error)
	History(Identity) ([]HistoryEntry, error)
	Wait([]WatchTarget, time.Duration) ([]Change, error)
	Close() error
}
