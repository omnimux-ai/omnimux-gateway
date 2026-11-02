package hostedaccount

import "github.com/QuantumNous/new-api/model"

var accountTransitions = map[string][]string{
	model.HostedAccountProcuring:   {model.HostedAccountAvailable, model.HostedAccountQuarantined, model.HostedAccountSuspended},
	model.HostedAccountAvailable:   {model.HostedAccountReserved, model.HostedAccountQuarantined, model.HostedAccountSuspended, model.HostedAccountRecycled},
	model.HostedAccountReserved:    {model.HostedAccountAssigned, model.HostedAccountAvailable, model.HostedAccountQuarantined, model.HostedAccountSuspended},
	model.HostedAccountAssigned:    {model.HostedAccountWarmingUp, model.HostedAccountQuarantined, model.HostedAccountSuspended},
	model.HostedAccountWarmingUp:   {model.HostedAccountReady, model.HostedAccountQuarantined, model.HostedAccountSuspended},
	model.HostedAccountReady:       {model.HostedAccountActive, model.HostedAccountQuarantined, model.HostedAccountSuspended},
	model.HostedAccountActive:      {model.HostedAccountQuarantined, model.HostedAccountSuspended},
	model.HostedAccountSuspended:   {model.HostedAccountReady, model.HostedAccountActive, model.HostedAccountRecycled, model.HostedAccountQuarantined},
	model.HostedAccountQuarantined: {model.HostedAccountRecycled},
}

func CanTransitionAccount(from, to string) bool {
	for _, next := range accountTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

func TransitionAccount(id int64, from, to string) error {
	if !CanTransitionAccount(from, to) {
		return ErrInvalidRequest
	}
	return model.TransitionHostedAccount(id, from, to)
}

func ChannelCanDistribute(status string) bool {
	return status == model.HostedAccountReady || status == model.HostedAccountActive
}

func ChannelUserFacingBlocked(status string) error {
	switch status {
	case model.HostedAccountSuspended:
		return ErrChannelSuspended
	case model.HostedAccountQuarantined, model.HostedAccountRecycled:
		return ErrChannelReplacing
	case model.HostedAccountReady, model.HostedAccountActive:
		return nil
	default:
		return ErrChannelNotReady
	}
}
