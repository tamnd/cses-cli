package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) problemsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "problems",
		Short: "List all CSES problems",
		Long: `List every problem from the CSES Problem Set.

Each record includes the problem's rank, ID, title, category, and URL.
Use --output to change the format and --limit to cap the number of records.

Examples:
  cses problems
  cses problems --limit 20
  cses problems --output json
  cses problems --fields rank,title,url`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(0)
			a.progressf("fetching problem list...")
			problems, err := a.client.Problems(cmd.Context(), n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(problems, len(problems))
		},
	}
}
