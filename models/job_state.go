package models

import (
	"scheduler0/constants"
)

type JobStateReqPayload struct {
	State constants.Command
	Data  []*JobModel
}
