package dbot

var gXManager = NewXManager()

type XManager struct {
	runnerMap map[string]Runner
}

func NewXManager() *XManager {
	ret := &XManager{
		runnerMap: make(map[string]Runner),
	}

	ret.runnerMap["local"] = &LocalRunner{}

	return ret
}

func (p *XManager) GetRunner(id string) Runner {
	if v, ok := p.runnerMap[id]; ok {
		return v
	}

	return nil
}

func (p *XManager) SetRunner(id string, runner Runner) {
	p.runnerMap[id] = runner
}
