package domain

type Feed interface {
	Start(history, isLive bool, strats []Strategy)
	AddStore(store Store)
}
