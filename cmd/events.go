package cmd

import (
	"github.com/spf13/cobra"

	"github.com/Bnei-Baruch/archive-backend/events"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "MDB events listener",
	Run:   eventsFn,
}

func init() {
	RootCmd.AddCommand(eventsCmd)
}

func eventsFn(cmd *cobra.Command, args []string) {
	events.RunListener()
}
