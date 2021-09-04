package context

type ImportContext struct {
	BaseContext
}


func (p *ImportContext) Run() bool {
	return true
}

func (p *ImportContext) GetSSHGroup(name string) []*Remote {
	config := make(map[string][]*Remote)

	if !p.LoadConfig(&config) {
		return nil
	} else if ret, ok := config[name]; !ok {
		p.Clone(name).LogError("SSHGroup \"%s\" not found", name)
		return nil
	} else if len(ret) == 0 {
		p.Clone(name).LogError("SSHGroup \"%s\" is empty", name)
		return nil
	} else {
		return ret
	}
}
