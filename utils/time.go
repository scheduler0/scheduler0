package utils

import (
	"sync"
	"time"
)

type SchedulerTime struct {
	loc *time.Location
}

var cachedSchedulerTime *SchedulerTime
var once sync.Once

func GetSchedulerTime() *SchedulerTime {
	if cachedSchedulerTime != nil {
		return cachedSchedulerTime
	}

	once.Do(func() {
		cachedSchedulerTime = &SchedulerTime{}
	})

	return cachedSchedulerTime
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
