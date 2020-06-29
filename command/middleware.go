package command

import (
	"fmt"

	"github.com/spf13/cobra"
)

type CobraFunc func(cmd *cobra.Command, args []string) error

type Func func(CobraFunc) CobraFunc

func Errorf(message string, args ...interface{}) CobraFunc {
	return func(*cobra.Command, []string) error {
		return fmt.Errorf(message, args...)
	}
}

func Use(cmd *cobra.Command, mw Func) {
	var apply func(*cobra.Command)
	apply = func(cmd *cobra.Command) {
		run := cmd.RunE
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			return mw(run)(cmd, args)
		}

		for _, cmd := range cmd.Commands() {
			apply(cmd)
		}
	}
	apply(cmd)
}
