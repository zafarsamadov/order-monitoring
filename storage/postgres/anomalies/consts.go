package anomalies

// Bir nechta qoida ishlatadigan qiymatlar. Faqat bitta qoidaga tegishli konstantalar o'z faylida turadi.

// Severity.
const (
	severityHigh   = "high"
	severityMedium = "medium"
	severityLow    = "low"
)

// Bazadagi status qiymatlari (migration/up/00001.up.sql dagi izohlarga qarang).
const (
	issuedToBuyer     = "ISSUED_TO_BUYER"
	returnedToSellers = "RETURNED_TO_SELLERS"
	returnToHub       = "RETURN_TO_HUB"
	dispatched        = "DISPATCHED"
	received          = "RECEIVED"
	inTransit         = "IN_TRANSIT"
)
