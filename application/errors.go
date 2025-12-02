package application

type Error string

func (e Error) Error() string {
	return string(e)
}

const (
	ErrDatabaseNil          = Error("database is nil")
	ErrMissingParameters    = Error("missing parameters")
	ErrDatabaseNotAvailable = Error("database not available")

	// Bridge errors
	ErrBridgeNotFound      = Error("bridge transaction not found")
	ErrInvalidDestChain    = Error("invalid destination chain")
	ErrBridgeAlreadyExists = Error("bridge transaction already exists")
	ErrInvalidBridgeID     = Error("invalid bridge ID")
	ErrBridgeNotPending    = Error("bridge transaction is not pending")
	ErrInvalidChainID      = Error("invalid chain ID")
)
