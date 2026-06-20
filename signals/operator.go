package signals

type Operator any

type op struct{}

func (o op) And() Operator {
	return o
}

func (o op) Or() Operator {
	return nil
}
