package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/jparrill/auriga-cli/internal/config"
)

type OrderedParam struct {
	Key   string
	Value string
}

func ConfirmOperationOrdered(title string, params []OrderedParam, command string, skipConfirm bool) (bool, error) {
	var sb strings.Builder
	sb.WriteString(BoldStyle.Render(title) + "\n\n")
	for _, p := range params {
		sb.WriteString(FormatKeyValue(p.Key, p.Value) + "\n")
	}
	if command != "" {
		sb.WriteString("\n" + MutedStyle.Render("Command: "+command))
	}
	fmt.Println(SummaryBox.Render(sb.String()))

	if skipConfirm || config.Yes {
		return true, nil
	}

	var confirmed bool
	err := huh.NewConfirm().
		Title("Proceed?").
		Value(&confirmed).
		Run()
	return confirmed, err
}
