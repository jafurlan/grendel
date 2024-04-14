package pages

type GlobalState struct {
	Authenticated bool
	User          string
	Role          string
}

func NewGlobalState() *GlobalState {
	return &GlobalState{}
}
