package camoufox

func (p ProxyConfig) AsString() string {
	if p.Username == "" && p.Password == "" {
		return p.Server
	}
	return p.Server
}
