package models

type RuleResponse struct {
	TotalOrders int        `json:"total_orders"`
	Items       []RuleItem `json:"items"`
}

type RuleItem struct {
	Rule        string `json:"rule"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Total       int    `json:"total"`
	Items       []Item `json:"items"`
}

type Item struct {
	OrderCode string `json:"order_code"`
	Reason    string `json:"reason"`
}

type Anomaly struct {
	Rule      string `json:"rule"`
	Severity  string `json:"severity"`
	OrderID   string `json:"-"` // orderga bog'lash uchun (timeline), javobga chiqmaydi
	OrderCode string `json:"order_code,omitempty"`
	Reason    string `json:"reason"` // qaysi maydon qanday qiymatda
}
