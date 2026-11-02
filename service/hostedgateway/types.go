package hostedgateway

// HealthStatus is GET /api/v1/health (anonymous).
type HealthStatus struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// QueueStatus is the jobs domain DTO (no code:0 envelope).
type QueueStatus struct {
	CfgCap             int  `json:"cfgCap"`
	EffectiveCap       int  `json:"effectiveCap"`
	PlanCap            int  `json:"planCap"`
	SelfActive         int  `json:"selfActive"`
	OccupiedBound      int  `json:"occupiedBound"`
	Queued             int  `json:"queued"`
	Running            int  `json:"running"`
	RecoveryRequired   int  `json:"recoveryRequired"`
	QuarantinedSeats   int  `json:"quarantinedSeats"`
	PlanCacheAgeSec    int  `json:"planCacheAgeSec"`
	JobsEnabled        bool `json:"jobsEnabled"`
}

// DeviceGuardResult is POST /deviceguard/check success shape.
type DeviceGuardResult struct {
	OK      bool   `json:"ok"`
	PhoneID string `json:"phoneId"`
}

// Envelope is the upstream-style business wrapper.
type Envelope struct {
	TraceID string `json:"traceId"`
	Code    int    `json:"code"`
	Msg     string `json:"msg"`
	Hint    string `json:"hint"`
	Data    any    `json:"data"`
}

// TaskAddResult is parsed from task/add data.
type TaskAddResult struct {
	TaskIDs []string
	Raw     any
}

// Job is the domain jobs DTO (create/get).
type Job struct {
	JobID          string `json:"jobId"`
	Status         string `json:"status"`
	PhoneID        string `json:"phoneId"`
	FlowID         string `json:"flowId"`
	IdempotencyKey string `json:"idempotencyKey"`
	CleanupResult  string `json:"cleanupResult"`
	ErrorCode      string `json:"errorCode"`
	ErrorMsg       string `json:"errorMsg"`
}

// AnalyticsAccount is one registered analytics row.
type AnalyticsAccount struct {
	ID      string `json:"id"`
	Account string `json:"account"`
	Channel int    `json:"channel"`
	Remark  string `json:"remark"`
}

// AnalyticsAccountsPage is analytics/accounts/list data.
type AnalyticsAccountsPage struct {
	Items []AnalyticsAccount `json:"items"`
	Total int                `json:"total"`
}

// AnalyticsRow is one daily aggregate. PlayCountStale is true when
// the upstream sent playCount=-1 (not yet updated).
type AnalyticsRow struct {
	Account       string `json:"account"`
	Channel       int    `json:"channel"`
	PlayCount     int64  `json:"playCount"`
	PlayCountStale bool  `json:"-"`
	FollowerCount int64  `json:"followerCount"`
	CommentCount  int64  `json:"commentCount"`
	DiggCount     int64  `json:"diggCount"`
	DataDate      int64  `json:"dataDate"`
}

// AnalyticsDataPage is analytics/data data.
type AnalyticsDataPage struct {
	Items []AnalyticsRow `json:"items"`
	Total int            `json:"total"`
}

// PhoneListPage is a thin phone/list wrapper; items stay generic maps
// so Phase 2 can pick group/countryName without a second decode pass.
type PhoneListPage struct {
	Items []map[string]any `json:"items"`
	Total int              `json:"total"`
}

// MaterialUploadURL is material/getUploadUrl data.
type MaterialUploadURL struct {
	UploadURL   string `json:"uploadUrl"`
	ResourceURL string `json:"resourceUrl"`
}
