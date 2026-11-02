package modelcontracts

import (
	"errors"
	"fmt"
)

// errNoModelID is returned for a spec file that does not name the model it
// describes; without an id it cannot be keyed into the payload.
var errNoModelID = errors.New("spec has no model_id")

// SpecError names the file that failed to project, so a regeneration failure
// points at the file to fix rather than at the whole directory.
type SpecError struct {
	File string
	Err  error
}

func (e *SpecError) Error() string {
	return fmt.Sprintf("modelcontracts: %s: %v", e.File, e.Err)
}

func (e *SpecError) Unwrap() error { return e.Err }
