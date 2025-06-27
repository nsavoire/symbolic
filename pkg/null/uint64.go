package null

// Uint64 is an nullable uint64 value
type Uint64 struct {
	Valid bool
	Value uint64
}

// ValidUint64 returns a valid Uint64 with the provided value.
// It's just a shortcut for creating the Uint64{} directly.
func ValidUint64(value uint64) Uint64 {
	return Uint64{Valid: true, Value: value}
}

// Is checks if the value is set and equal to the provided value
func (n Uint64) Is(v uint64) bool {
	return n.Valid && n.Value == v
}
