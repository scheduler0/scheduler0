package models

type DateRangeAnalyticsPoint struct {
	Date      string `json:"date"`
	Time      string `json:"time"`
	Scheduled uint64 `json:"scheduled"`
	Success   uint64 `json:"success"`
	Failed    uint64 `json:"failed"`
}

type DateRangeAnalyticsResponse struct {
	AccountID uint64                    `json:"accountId"`
	Timezone  string                    `json:"timezone"`
	StartDate string                    `json:"startDate"`
	StartTime string                    `json:"startTime"`
	EndDate   string                    `json:"endDate"`
	EndTime   string                    `json:"endTime"`
	Points    []DateRangeAnalyticsPoint `json:"points"`
}
