package rule

type Rule interface {
}

type rule struct {
}

func NewRule() Rule {
	return &rule{}
}

func (s *rule) Satisfied() {

}
