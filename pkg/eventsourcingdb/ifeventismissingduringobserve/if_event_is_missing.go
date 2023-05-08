package ifeventismissingduringobserve

type IfEventIsMissing string

const (
	ReadEverything IfEventIsMissing = "read-everything"
	WaitForEvent   IfEventIsMissing = "wait-for-event"
)
