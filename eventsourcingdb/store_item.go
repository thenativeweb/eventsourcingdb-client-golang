package eventsourcingdb

import "github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb/event"

type StoreItem struct {
	Hash  string      `json:"hash"`
	Event event.Event `json:"event"`
}
