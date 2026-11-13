package hostedaccount

import "errors"

var (
	ErrGatewayUnavailable   = errors.New("hosted gateway unavailable")
	ErrChannelNotReady      = errors.New("channel not ready")
	ErrChannelSuspended     = errors.New("channel suspended")
	ErrChannelReplacing     = errors.New("channel replacing")
	ErrChannelNotFound      = errors.New("channel not found")
	ErrOrderNotFound        = errors.New("order not found")
	ErrTaskNotFound         = errors.New("task not found")
	ErrNotReadyToDistribute = errors.New("channel cannot distribute yet")
	ErrInvalidRequest       = errors.New("invalid hosted request")
	ErrIdempotentConflict   = errors.New("hosted idempotency conflict")
	ErrOrderLimit           = errors.New("undelivered hosted quantity exceeds limit")
	ErrSKUDisabled          = errors.New("hosted sku disabled")
	ErrBalanceInsufficient  = errors.New("余额不足")
	ErrReservedPhone        = errors.New("reserved hosted phone cannot be allocated")
	ErrCredKeyMissing       = errors.New("hosted credential key missing")
	ErrCredInvalid          = errors.New("hosted credential envelope invalid")
)
