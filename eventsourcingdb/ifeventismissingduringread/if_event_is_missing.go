package ifeventismissingduringread

type IfEventIsMissing string

const (
	ReadEverything IfEventIsMissing = "read-everything"
	ReadNothing    IfEventIsMissing = "read-nothing"
)
