package based

import (
	"github.com/go-playground/validator/v10"
)

// Validate is a default shared validator.Validate instance.
var Validate = validator.New()

// Unit represents an empty type, similar to Unit in Scala.
type Unit struct{}

// Nop is an empty function.
func Nop() {}
