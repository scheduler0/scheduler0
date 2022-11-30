package models

import (
	"scheduler0/constants"
)

type JobStateReqPayload struct {
	ServerAddress string
	State         constants.Command
	Data          []JobModel
}
