package layout

type State struct {
	Expanded bool
	Aside bool
	Dock bool
}

func Mode(s State) string {
	switch {
	case !s.Expanded:
		return "collapsed"
	case s.Dock:
		return "dock"
	default:
		return "float"
	}
}
