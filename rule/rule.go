package rule

type Rule any

type rule struct {
}

func NewRule() Rule {
	return &rule{}
}

func (s *rule) Satisfied() {

}
