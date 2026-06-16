package ui

import (
	"github.com/charmbracelet/huh/spinner"
)

func WithSpinner(title string, fn func() error) error {
	var actionErr error
	if err := spinner.New().
		Title(title).
		Action(func() {
			actionErr = fn()
		}).
		Run(); err != nil {
		return err
	}
	return actionErr
}
