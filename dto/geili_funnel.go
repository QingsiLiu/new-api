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
	UserID      int    `json:"user_id,omitempty"`
}
