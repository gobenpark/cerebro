package strategy

import "fmt"

type SampleStrategy struct{}

func (s *SampleStrategy) Next() {
	fmt.Println("next called")
}

func (s *SampleStrategy) NotifyOrder() {
	panic("implement me")
}

func (s *SampleStrategy) NotifyTrade() {
	panic("implement me")
}

func (s *SampleStrategy) NotifyCashValue() {
	panic("implement me")
}

func (s *SampleStrategy) NotifyFund() {
	panic("implement me")
}
