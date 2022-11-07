package eventsourcingdb

type StoreItem struct {
	Hash  string `json:"hash"`
	Event Event  `json:"event"`
}
