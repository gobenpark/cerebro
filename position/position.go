package position

import "time"

type Position struct {
	Size      int64     `json:"size"`
	Price     float64   `json:"price"`
	CreatedAt time.Time `json:"createdAt"`
}
