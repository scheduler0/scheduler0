package fsm

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"log"
	"net/http"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/marsher"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"strconv"
	"time"
)

func AppApply(logger *log.Logger, rft *raft.Raft, commandType constants.Command, sqlString string, params []interface{}) (*Response, *utils.GenericError) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommand := &protobuffs.Command{
		Type: protobuffs.Command_Type(commandType),
		Sql:  sqlString,
		Data: data,
	}

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommandData, err := marsher.MarshalCommand(createCommand)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	configs := config.GetScheduler0Configurations(logger)

	timeout, err := strconv.Atoi(configs.RaftApplyTimeout)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	af := rft.Apply(createCommandData, time.Second*time.Duration(timeout)).(raft.ApplyFuture)
	if af.Error() != nil {
		if af.Error() == raft.ErrNotLeader {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, "server not raft leader")
		}
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, af.Error().Error())
	}

	if af.Response() != nil {
		r := af.Response().(Response)
		if r.Error != "" {
			return nil, utils.HTTPGenericError(http.StatusInternalServerError, r.Error)
		}
		return &r, nil
	}

	return nil, nil
}
