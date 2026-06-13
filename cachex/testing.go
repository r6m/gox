package cachex

// Fake is an in-memory cache intended for deterministic application tests.
type Fake struct {
	*Memory
}

// NewFake creates an empty cache fake.
func NewFake() *Fake {
	return &Fake{Memory: NewMemory()}
}

var _ Cache = (*Fake)(nil)
