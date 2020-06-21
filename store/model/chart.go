package model

type Chart struct {
	Code   string  `json:"code" validate:"required"`
	Low    float64 `json:"low" validate:"required"`
	High   float64 `json:"high" validate:"required"`
	Open   float64 `json:"open" validate:"required"`
	Close  float64 `json:"close" validate:"required"`
	Volume float64 `json:"volume" validate:"required"`
	Date   string  `json:"date" validate:"required"`
}
