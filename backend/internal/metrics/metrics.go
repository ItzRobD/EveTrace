package metrics

import "sync/atomic"

var (
	EventsProcessed atomic.Int64
	SessionsOpened  atomic.Int64
	WSClients       atomic.Int32
)
