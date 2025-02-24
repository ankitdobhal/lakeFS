package cmd

import (
	"encoding/json"
	"io/ioutil"

	"github.com/treeverse/lakefs/pkg/api/gen/models"

	"github.com/spf13/cobra"
	"github.com/treeverse/lakefs/pkg/cmdutils"
	"github.com/treeverse/lakefs/pkg/uri"
)

var metadataDumpTemplate = `
{{ .Response | json }}
`

var refsRestoreSuccess = `
{{ "All references restored successfully!" | green }}
`

var refsRestoreCmd = &cobra.Command{
	Use:   "refs-restore <repository uri>",
	Short: "restores refs (branches, commits, tags) from the underlying object store to a bare repository",
	Long: `restores refs (branches, commits, tags) from the underlying object store to a bare repository.

This command is expected to run on a bare repository (i.e. one created with 'lakectl repo create-bare').
Since a bare repo is expected, in case of transient failure, delete the repository and recreate it as bare and retry.`,
	Example: "aws s3 cp s3://bucket/_lakefs/refs_manifest.json - | lakectl refs-load lakefs://my-bare-repository --manifest -",
	Hidden:  true,
	Args: cmdutils.ValidationChain(
		cobra.ExactArgs(1),
		cmdutils.FuncValidator(0, uri.ValidateRepoURI),
	),
	Run: func(cmd *cobra.Command, args []string) {
		repoURI := uri.Must(uri.Parse(args[0]))
		manifestFileName, _ := cmd.Flags().GetString("manifest")
		fp := OpenByPath(manifestFileName)
		defer func() {
			_ = fp.Close()
		}()

		// read and parse the JSON
		data, err := ioutil.ReadAll(fp)
		if err != nil {
			DieErr(err)
		}
		manifest := &models.RefsDump{}
		err = json.Unmarshal(data, manifest)
		if err != nil {
			DieErr(err)
		}

		// execute the restore operation
		client := getClient()
		err = client.RefsRestore(cmd.Context(), repoURI.Repository, manifest)
		if err != nil {
			DieErr(err)
		}
		Write(refsRestoreSuccess, nil)
	},
}

var refsDumpCmd = &cobra.Command{
	Use:    "refs-dump <repository uri>",
	Short:  "dumps refs (branches, commits, tags) to the underlying object store",
	Hidden: true,
	Args: cmdutils.ValidationChain(
		cobra.ExactArgs(1),
		cmdutils.FuncValidator(0, uri.ValidateRepoURI),
	),
	Run: func(cmd *cobra.Command, args []string) {
		repoURI := uri.Must(uri.Parse(args[0]))

		client := getClient()
		resp, err := client.RefsDump(cmd.Context(), repoURI.Repository)
		if err != nil {
			DieErr(err)
		}

		Write(metadataDumpTemplate, struct {
			Response interface{}
		}{resp})
	},
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(refsDumpCmd)
	rootCmd.AddCommand(refsRestoreCmd)

	refsRestoreCmd.Flags().String("manifest", "", "path to a refs manifest json file (as generated by `refs-dump`). Alternatively, use \"-\" to read from stdin")
	_ = refsRestoreCmd.MarkFlagRequired("manifest")
}
