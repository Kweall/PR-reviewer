package service

import "errors"

var (
	ErrNotFound       = errors.New("not found")
	ErrPRExists       = errors.New("pr exists")
	ErrPRMerged       = errors.New("pr merged")
	ErrNotAssigned    = errors.New("not assigned")
	ErrNoCandidate    = errors.New("no candidate")
	ErrUnknownJobType = errors.New("unknown job type")
	ErrJobQueueFull   = errors.New("job queue full")
)
