package models

import (
	"encoding/json"
	"net/url"
	"time"
)

// Record — bitta API yozuvining bazaga upsert qilishga tayyor ko'rinishi.
// Table va Cols kod ichidagi konstantalardan keladi, foydalanuvchi kiritmaydi.
type Record struct {
	Table  string
	KeyCol string
	Key    string
	Cols   []string
	Vals   []any
	Raw    []byte // kanonik JSON
	Hash   string // Raw hash'i: o'zgarmasa, yozuv "unchanged" hisoblanadi
}

// UpsertResult — bitta yozuv upsert natijasi.
type UpsertResult int

const (
	UpsertUnchanged UpsertResult = iota // raw_hash o'zgarmagan
	UpsertInserted                      // yangi yozuv
	UpsertUpdated                       // raw_hash o'zgargan
)

// PagedEntity — stenddagi limit/offset bilan sahifalanadigan ro'yxat va uni Record'ga o'girish.
type PagedEntity struct {
	Name     string
	Path     string
	ItemsKey string // javobdagi massiv kaliti
	Query    url.Values
	ToRecord func(json.RawMessage) (Record, error)
}

type UpsertStats struct {
	Inserted  int `json:"inserted"`
	Updated   int `json:"updated"`
	Unchanged int `json:"unchanged"`
}

func (s *UpsertStats) Add(o UpsertStats) {
	s.Inserted += o.Inserted
	s.Updated += o.Updated
	s.Unchanged += o.Unchanged
}

const (
	SyncRunning = "running"
	SyncSuccess = "success"
	SyncFailed  = "failed"
)

// EntityResult — bitta obyekt turi (orders, packages, ...) bo'yicha sync natijasi.
type EntityResult struct {
	Entity     string `json:"entity"`
	Fetched    int    `json:"fetched"`
	Skipped    int    `json:"skipped"`
	APITotal   *int   `json:"api_total"`
	LastOffset int    `json:"last_offset"`
	UpsertStats
}

// SyncRun — sync_runs jadvalidagi bitta ishga tushirish.
type SyncRun struct {
	ID         int64          `json:"id"`
	Status     string         `json:"status"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at"`
	LastEntity string         `json:"last_entity"`
	LastOffset int            `json:"last_offset"`
	Error      string         `json:"error,omitempty"`
	Entities   []EntityResult `json:"entities"`
	UpsertStats
}

// Recount umumiy sonlarni obyekt turlari bo'yicha qayta hisoblaydi.
func (r *SyncRun) Recount() {
	r.UpsertStats = UpsertStats{}
	for _, e := range r.Entities {
		r.UpsertStats.Add(e.UpsertStats)
	}
}
