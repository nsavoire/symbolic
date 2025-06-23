package null // import "github.com/DataDog/symbolic/null"

// String is an nullable string value
type String struct {
	Valid bool
	Value string
}

// ValidString returns a valid String with the provided value.
// It's just a shortcut for creating the String{} directly.
func ValidString(value string) String {
	return String{Valid: true, Value: value}
}

// Is checks if the value is set and equal to the provided value
func (n String) Is(v string) bool {
	return n.Valid && n.Value == v
}
