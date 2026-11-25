package models

type RuleResponse struct {
	TotalOrders int        `json:"total_orders"` // ko'rsatilgan qoidalar bo'yicha noyob orderlar
	Items       []RuleItem `json:"items"`
}

type RuleItem struct {
	Rule        string `json:"rule"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Total       int    `json:"total"`    // noyob orderlar soni
	Examples    []Item `json:"examples"` // 3 tagacha, har biri boshqa order
	Items       []Item `json:"items"`
}

type Item struct {
	OrderCode string `json:"order_code"`
	Reason    string `json:"reason"`
}

// Anomaly — timeline'dagi bitta anomaliya.
type Anomaly struct {
	Rule      string `json:"rule"`
	Severity  string `json:"severity"`
	OrderCode string `json:"order_code"`
	Reason    string `json:"reason"` // qaysi maydon qanday qiymatda
}
