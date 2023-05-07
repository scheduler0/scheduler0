package fsm

import (
	"encoding/json"
	"github.com/hashicorp/raft"
	"google.golang.org/protobuf/proto"
	"net/http"
	"scheduler0/config"
	"scheduler0/constants"
	"scheduler0/protobuffs"
	"scheduler0/utils"
	"time"
)

func AppApply(rft *raft.Raft, commandType constants.Command, sqlString string, nodeId uint64, params []interface{}) (*Response, *utils.GenericError) {
	data, err := json.Marshal(params)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommand := &protobuffs.Command{
		Type:       protobuffs.Command_Type(commandType),
		Sql:        sqlString,
		Data:       data,
		TargetNode: nodeId,
	}

	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	createCommandData, err := proto.Marshal(createCommand)
	if err != nil {
		return nil, utils.HTTPGenericError(http.StatusInternalServerError, err.Error())
	}

	configs := config.NewScheduler0Config().GetConfigurations()

	af := rft.Apply(createCommandData, time.Second*time.Duration(configs.RaftApplyTimeout)).(raft.ApplyFuture)
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
