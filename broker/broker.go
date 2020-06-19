package broker

type Broker interface {
	Buy()
	Sell()
	Cancel()
	Submit()
	GetPosition()
}

type Broke struct {
	cash      int32
	commision float32
}
