package gateway

// Service owns gateway-route control-plane state.
type Service interface {
	Get(Identity) (*Route, error)
	List(ListQuery) (ListResult, error)
	Create(CreateRequest) (*Route, error)
	Update(UpdateRequest) (*Route, error)
	Delete(Identity, uint64, string) (*HistoryEntry, error)
	SetEnabled(Identity, bool, uint64, string) (*Route, error)
	History(Identity) ([]HistoryEntry, error)
	Subscribe(func()) func()
	Close() error
}
