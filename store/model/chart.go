package model

type Chart struct {
	Code   string `json:"code" validate:"required"`
	Low    int    `json:"low" validate:"required"`
	High   int    `json:"high" validate:"required"`
	Open   int    `json:"open" validate:"required"`
	Close  int    `json:"close" validate:"required"`
	Volume int    `json:"volume" validate:"required"`
}
