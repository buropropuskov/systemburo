package realtime

import "errors"

const (
	defaultOwnerSubscriptions  = 8
	defaultSystemSubscriptions = 200
	defaultOwnerTickets        = 16
	defaultSystemTickets       = 400
)

// Ошибки допуска общие для Hub и TicketStore, совместимы с errors.Is.
var (
	ErrOwnerLimit      = errors.New("realtime owner limit reached")
	ErrSystemLimit     = errors.New("realtime system limit reached")
	ErrClosed          = errors.New("realtime closed")
	ErrInvalidIdentity = errors.New("invalid session identity")
)
