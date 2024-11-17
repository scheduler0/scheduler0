package models

import "scheduler0/pkg/constants"

type PostProcess struct {
	Action      constants.CommandAction
	TargetNodes []uint64
	Data        SQLResponse
}
