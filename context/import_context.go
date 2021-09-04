package context

type ImportContext struct {
	importConfig map[string][]*Remote
	BaseContext
}

func (p *ImportContext) Clone(pathFormat string, a ...interface{}) Context {
	return &ImportContext{
		importConfig: p.importConfig,
		BaseContext:  *p.BaseContext.copy(pathFormat, a...),
	}
}

func (p *ImportContext) Run() bool {
	return p.LoadConfig(p.BaseContext.config, &p.importConfig)
}

func (p *ImportContext) GetSSHGroup(name string) []*Remote {
	ret, ok := p.importConfig[name]

	if !ok {
		p.Clone("%s", name).LogErrorf("SSHGroup \"%s\" not found", name)
		return nil
	}

	if len(ret) == 0 {
		p.Clone("%s", name).LogErrorf("SSHGroup \"%s\" is empty", name)
		return nil
	}

	return ret
}
