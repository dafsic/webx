package errors

import (
	"fmt"

	"github.com/pkg/errors"
)

type TemporaryTablesError struct {
	err error
}

func NewTemporaryTablesError(description error) error {
	return errors.WithStack(&TemporaryTablesError{
		err: description,
	})
}

func (e *TemporaryTablesError) Error() string {
	return fmt.Sprintf("permissions adapter error: %s", e.err)
}

type ValidationError struct {
	parameterName string
	parameterErr  error
}

func NewValidationError(parameterName string, parameterErr error) error {
	return errors.WithStack(&ValidationError{
		parameterName: parameterName,
		parameterErr:  parameterErr,
	})
}

func (e *ValidationError) Error() string {
	return errors.Wrapf(
		e.parameterErr,
		"validation error. problem with parameter [%s]",
		e.parameterName).Error()
}

type NilError struct {
	nilObject string
}

func NewNilError(nilObject string) error {
	return errors.WithStack(&NilError{nilObject})
}

func (e *NilError) Error() string {
	return fmt.Sprintf("cannot be nil: %s", e.nilObject)
}
