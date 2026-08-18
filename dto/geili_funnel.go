package dto

type GeiliFunnelEventRequest struct {
	EventID     string `json:"event_id"`
	Event       string `json:"event"`
	Version     int    `json:"version"`
	Environment string `json:"environment"`
	VisitorHMAC string `json:"visitor_hmac"`
	Locale      string `json:"locale,omitempty"`
	Model       string `json:"model,omitempty"`
	FailureCode string `json:"failure_code,omitempty"`
	SourceHost  string `json:"source_host,omitempty"`
	UserID      int    `json:"user_id,omitempty"`
}

type FunnelWindow struct {
	From                string `json:"from"`
	To                  string `json:"to"`
	Timezone            string `json:"timezone"`
	Environment         string `json:"environment"`
	CollectionStartedAt int64  `json:"collection_started_at"`
	RawCutoff           int64  `json:"raw_cutoff"`
}

type FunnelCount struct {
	People       *int64   `json:"people"`
	RatePrevious *float64 `json:"rate_previous"`
	RateEntry    *float64 `json:"rate_entry"`
	Suppressed   bool     `json:"suppressed"`
}

type FunnelStage struct {
	Name string `json:"name"`
	FunnelCount
}

type FunnelStrict struct {
	Coverage string        `json:"coverage"`
	Stages   []FunnelStage `json:"stages"`
}

type FunnelMilestone struct {
	Name       string `json:"name"`
	People     *int64 `json:"people"`
	Ordered    bool   `json:"ordered"`
	Coverage   string `json:"coverage"`
	Suppressed bool   `json:"suppressed"`
}

type FunnelRetention struct {
	Day        int      `json:"day"`
	Eligible   *int64   `json:"eligible"`
	Retained   *int64   `json:"retained"`
	Rate       *float64 `json:"rate"`
	Immature   *int64   `json:"immature"`
	Coverage   string   `json:"coverage"`
	Suppressed bool     `json:"suppressed"`
}

type FunnelFailure struct {
	Code       string `json:"code"`
	Model      string `json:"model,omitempty"`
	Count      *int64 `json:"count"`
	Coverage   string `json:"coverage"`
	Suppressed bool   `json:"suppressed"`
}

type FunnelMetrics struct {
	Strict      FunnelStrict      `json:"strict_funnel"`
	Independent []FunnelMilestone `json:"independent_milestones"`
	Retention   []FunnelRetention `json:"retention"`
	Failures    []FunnelFailure   `json:"failures"`
}

type FunnelSegment struct {
	Dimension string        `json:"dimension"`
	Value     string        `json:"value"`
	Metrics   FunnelMetrics `json:"metrics"`
}

type FunnelQuality struct {
	Unlinked            int64  `json:"unlinked"`
	Ambiguous           int64  `json:"ambiguous"`
	DuplicateSinceStart uint64 `json:"duplicate_since_process_start"`
	RejectedSinceStart  uint64 `json:"rejected_since_process_start"`
	CounterSince        int64  `json:"counter_since_process_start"`
	LastEventAt         int64  `json:"last_event_at"`
	InvalidTopUpTimes   int64  `json:"invalid_top_up_times"`
	InvalidTaskTimes    int64  `json:"invalid_task_times"`
	InvalidTokenTimes   int64  `json:"invalid_token_times"`
	InvalidTextLogTimes int64  `json:"invalid_text_log_times"`
	SuppressedSegments  int64  `json:"suppressed_segments"`
}

type GeiliFunnelSummaryResponse struct {
	Window   FunnelWindow    `json:"window"`
	Metrics  FunnelMetrics   `json:"metrics"`
	Quality  FunnelQuality   `json:"quality"`
	Segments []FunnelSegment `json:"segments"`
}

type FunnelHealthEvent struct {
	Name    string `json:"name"`
	Last24h int64  `json:"last_24h"`
}

type FunnelHealthCounters struct {
	Accepted  uint64 `json:"accepted_since_process_start"`
	Duplicate uint64 `json:"duplicate_since_process_start"`
	Rejected  uint64 `json:"rejected_since_process_start"`
	Failed    uint64 `json:"failed_since_process_start"`
	Since     int64  `json:"since_process_start"`
}

type FunnelHealthIdentity struct {
	Linked        int64    `json:"linked"`
	Ambiguous     int64    `json:"ambiguous"`
	AmbiguousRate *float64 `json:"ambiguous_rate"`
}

type FunnelHealthBusiness struct {
	InvalidTopUpTimes          int64 `json:"invalid_top_up_times"`
	InvalidTaskTimes           int64 `json:"invalid_task_times"`
	InvalidTokenTimes          int64 `json:"invalid_token_times"`
	InvalidTextLogTimes        int64 `json:"invalid_text_log_times"`
	FirstAPIKeysLast24h        int64 `json:"first_api_keys_last_24h"`
	FirstActivatedLast24h      int64 `json:"first_activated_last_24h"`
	FirstSuccessfulTextLast24h int64 `json:"first_successful_text_last_24h"`
	TaskSuccessLast24h         int64 `json:"task_success_last_24h"`
	TaskFailureLast24h         int64 `json:"task_failure_last_24h"`
}

type FunnelHealthMaintenance struct {
	Status           string `json:"status"`
	LastRunAt        int64  `json:"last_run_at"`
	LastSuccessfulAt int64  `json:"last_successful_at"`
	RawCutoff        int64  `json:"raw_cutoff"`
	ActivityCutoff   int64  `json:"activity_cutoff"`
}

type GeiliFunnelHealthResponse struct {
	Healthy             bool                    `json:"healthy"`
	Enabled             bool                    `json:"enabled"`
	SchemaVersion       int                     `json:"schema_version"`
	Environment         string                  `json:"environment"`
	CollectionStartedAt int64                   `json:"collection_started_at"`
	LastEventAt         int64                   `json:"last_event_at"`
	Events              []FunnelHealthEvent     `json:"events"`
	Counters            FunnelHealthCounters    `json:"counters"`
	Identity            FunnelHealthIdentity    `json:"identity"`
	Business            FunnelHealthBusiness    `json:"business"`
	Maintenance         FunnelHealthMaintenance `json:"maintenance"`
}
