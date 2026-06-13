package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) searchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search CSES problems by title or category",
		Long: `Search CSES problems whose title or category contains <query>
(case-insensitive substring match).

Examples:
  cses search sorting
  cses search "dynamic programming"
  cses search graph --output json
  cses search tree --limit 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := a.effectiveLimit(0)
			q := args[0]
			a.progressf("searching for %q...", q)
			hits, err := a.client.Search(cmd.Context(), q, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(hits, len(hits))
		},
	}
}
