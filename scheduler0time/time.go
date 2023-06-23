package scheduler0time

import (
	"time"
)

type SchedulerTime struct {
	loc *time.Location
}

func GetSchedulerTime() *SchedulerTime {
	return &SchedulerTime{}
}

func (schedulerTime *SchedulerTime) SetTimezone(tz string) error {
	location, err := time.LoadLocation(tz)
	if err != nil {
		return err
	}
	schedulerTime.loc = location
	return nil
}

func (schedulerTime *SchedulerTime) GetTime(t time.Time) time.Time {
	if schedulerTime.loc == nil {
		return t
	}
	return t.In(schedulerTime.loc)
}
