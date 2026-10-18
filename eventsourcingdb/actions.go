package eventsourcingdb

// The actions that the errors of the client name, e.g. in "failed to write
// events, ...", and that DBAPIError.Action holds.
const (
	actionPing                = "ping"
	actionVerifyAPIToken      = "verify API token"
	actionWriteEvents         = "write events"
	actionRegisterEventSchema = "register event schema"
	actionReadEventType       = "read event type"
	actionReadEvents          = "read events"
	actionObserveEvents       = "observe events"
	actionRunEventQLQuery     = "run EventQL query"
	actionReadSubjects        = "read subjects"
	actionReadEventTypes      = "read event types"
)
