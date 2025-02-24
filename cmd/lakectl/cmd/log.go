package cmd

import (
	"github.com/go-openapi/swag"
	"github.com/spf13/cobra"
	"github.com/treeverse/lakefs/pkg/api/gen/models"
	"github.com/treeverse/lakefs/pkg/cmdutils"
	"github.com/treeverse/lakefs/pkg/uri"
)

const commitsTemplate = `
{{ range $val := .Commits }}
ID:            {{ $val.ID|yellow }}{{if $val.Committer }}
Author:        {{ $val.Committer }}{{end}}
Date:          {{ $val.CreationDate|date }}
{{ if $.ShowMetaRangeID }}Meta Range ID: {{ $val.MetaRangeID }}
{{ end -}}
{{ if gt ($val.Parents|len) 1 -}}
Merge:         {{ $val.Parents|join ", "|bold }}
{{ end }}
	{{ $val.Message }}
	
	{{ range $key, $value := $val.Metadata }}
		{{ $key }} = {{ $value }}
	{{ end -}}
{{ end }}
{{.Pagination | paginate }}
`

// logCmd represents the log command
var logCmd = &cobra.Command{
	Use:   "log <branch uri>",
	Short: "show log of commits for the given branch",
	Args: cmdutils.ValidationChain(
		cobra.ExactArgs(1),
		cmdutils.FuncValidator(0, uri.ValidateRefURI),
	),
	Run: func(cmd *cobra.Command, args []string) {
		amount, err := cmd.Flags().GetInt("amount")
		if err != nil {
			DieErr(err)
		}
		after, err := cmd.Flags().GetString("after")
		if err != nil {
			DieErr(err)
		}
		showMetaRangeID, _ := cmd.Flags().GetBool("show-meta-range-id")
		client := getClient()
		branchURI := uri.Must(uri.Parse(args[0]))
		commits, pagination, err := client.GetCommitLog(cmd.Context(), branchURI.Repository, branchURI.Ref, after, amount)
		if err != nil {
			DieErr(err)
		}
		ctx := struct {
			Commits         []*models.Commit
			Pagination      *Pagination
			ShowMetaRangeID bool
		}{
			Commits:         commits,
			ShowMetaRangeID: showMetaRangeID,
		}
		if pagination != nil && swag.BoolValue(pagination.HasMore) {
			ctx.Pagination = &Pagination{
				Amount:  amount,
				HasNext: true,
				After:   pagination.NextOffset,
			}
		}
		Write(commitsTemplate, ctx)
	},
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(logCmd)
	logCmd.Flags().Int("amount", -1, "how many results to return, or '-1' for default (used for pagination)")
	logCmd.Flags().String("after", "", "show results after this value (used for pagination)")
	logCmd.Flags().Bool("show-meta-range-id", false, "also show meta range ID")
}
