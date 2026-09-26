package models

import "scheduler0-private/pkg/constants"

type PostProcess struct {
	Action      constants.CommandAction
	TargetNodes []uint64
	Data        SQLResponse
}
