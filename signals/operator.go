package signals

type Operator interface{}

type op struct{}

func (o op) And() Operator {
	return o
}

func (o op) Or() Operator {
	return nil
}
