package broker

//go:generate mockgen -source=./broker.go -destination=./mock/mock_broker.go

type Broker interface {
	Buy()
	Sell()
	Cancel()
	Submit()
	GetPosition()
}

type broke struct {
	cash      int32
	commision float32
}

func NewBroker(cash int32, commision float32) Broker {
	return &broke{
		cash:      cash,
		commision: commision,
	}
}

func (b broke) Buy() {
	panic("implement me")
}

func (b broke) Sell() {
	panic("implement me")
}

func (b broke) Cancel() {
	panic("implement me")
}

func (b broke) Submit() {
	panic("implement me")
}

func (b broke) GetPosition() {
	panic("implement me")
}
