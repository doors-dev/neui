package layout

type State struct {
	Expanded bool
	Aside bool
	Dock bool
}

func Mode(s State, dock bool) string {
	switch {
	case !s.Expanded:
		return "collapsed"
	case s.Dock && dock:
		return "dock"
	default:
		return "float"
	}
}
