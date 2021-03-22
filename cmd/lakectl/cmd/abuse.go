package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/treeverse/lakefs/pkg/api"
	"github.com/treeverse/lakefs/pkg/cmdutils"
	"github.com/treeverse/lakefs/pkg/testutil/stress"
	"github.com/treeverse/lakefs/pkg/uri"
)

var ErrRequestFailed = errors.New("request failed")

var abuseCmd = &cobra.Command{
	Use:    "abuse <sub command>",
	Short:  "abuse a running lakeFS instance. See sub commands for more info.",
	Hidden: true,
}

func readLines(path string) ([]string, error) {
	reader := OpenByPath(path)
	scanner := bufio.NewScanner(reader)
	keys := make([]string, 0)
	for scanner.Scan() {
		keys = append(keys, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	err := reader.Close()
	if err != nil {
		return nil, err
	}
	return keys, nil
}

var abuseRandomReadsCmd = &cobra.Command{
	Use:    "random-read <source ref uri>",
	Short:  "Read keys from a file and generate random reads from the source ref for those keys.",
	Hidden: false,
	Args: cmdutils.ValidationChain(
		cobra.ExactArgs(1),
		cmdutils.FuncValidator(0, uri.ValidateRefURI),
	),
	Run: func(cmd *cobra.Command, args []string) {
		u := uri.Must(uri.Parse(args[0]))

		amount := MustInt(cmd.Flags().GetInt("amount"))
		parallelism := MustInt(cmd.Flags().GetInt("parallelism"))
		fromFile := MustString(cmd.Flags().GetString("from-file"))

		// read the input file
		keys, err := readLines(fromFile)
		if err != nil {
			DieErr(err)
		}
		Fmt("read a total of %d keys from key file\n", len(keys))

		generator := stress.NewGenerator(parallelism, stress.WithSignalHandlersFor(os.Interrupt, syscall.SIGTERM))

		// generate randomly selected keys as input
		rand.Seed(time.Now().Unix())
		generator.Setup(func(add stress.GeneratorAddFn) {
			for i := 0; i < amount; i++ {
				//nolint:gosec
				add(keys[rand.Intn(len(keys))])
			}
		})

		// execute the things!
		generator.Run(func(input chan string, output chan stress.Result) {
			ctx := cmd.Context()
			client := getClient()
			for work := range input {
				start := time.Now()
				resp, err := client.StatObjectWithResponse(ctx, u.Repository, u.Ref, &api.StatObjectParams{
					Path: work,
				})
				if err == nil && resp.StatusCode() != http.StatusOK {
					err = fmt.Errorf("%w: %s (%d)", ErrRequestFailed, resp.Status(), resp.StatusCode())
				}
				output <- stress.Result{
					Error: err,
					Took:  time.Since(start),
				}
			}
		})
	},
}

var abuseRandomWritesCmd = &cobra.Command{
	Use:    "random-write <source branch uri>",
	Short:  "Generate random writes to the source branch",
	Hidden: false,
	Args: cmdutils.ValidationChain(
		cobra.ExactArgs(1),
		cmdutils.FuncValidator(0, uri.ValidateRefURI),
	),
	Run: func(cmd *cobra.Command, args []string) {
		u := uri.Must(uri.Parse(args[0]))

		amount := MustInt(cmd.Flags().GetInt("amount"))
		parallelism := MustInt(cmd.Flags().GetInt("parallelism"))
		prefix := MustString(cmd.Flags().GetString("prefix"))

		generator := stress.NewGenerator(parallelism, stress.WithSignalHandlersFor(os.Interrupt, syscall.SIGTERM))

		// generate randomly selected keys as input
		rand.Seed(time.Now().Unix())
		generator.Setup(func(add stress.GeneratorAddFn) {
			for i := 0; i < amount; i++ {
				add(fmt.Sprintf("%sfile-%d", prefix, i))
			}
		})

		client := getClient()
		resp, err := client.GetRepositoryWithResponse(cmd.Context(), u.Repository)
		DieOnResponseError(resp, err)

		repo := resp.JSON200
		storagePrefix := repo.StorageNamespace
		var size int64
		var checksum = "00695c7307b0480c7b6bdc873cf05c15"
		addr := storagePrefix + "/random-write"
		creationInfo := api.ObjectStageCreation{
			Checksum:        checksum,
			PhysicalAddress: addr,
			SizeBytes:       size,
		}

		// execute the things!
		generator.Run(func(input chan string, output chan stress.Result) {
			ctx := cmd.Context()
			client := getClient()
			for work := range input {
				start := time.Now()
				resp, err := client.StageObjectWithResponse(ctx, u.Repository, u.Ref, &api.StageObjectParams{Path: work},
					api.StageObjectJSONRequestBody(creationInfo))
				if err == nil && resp.StatusCode() != http.StatusOK {
					err = fmt.Errorf("%w: %s (%d)", ErrRequestFailed, resp.Status(), resp.StatusCode())
				}
				output <- stress.Result{
					Error: err,
					Took:  time.Since(start),
				}
			}
		})
	},
}

var abuseCreateBranchesCmd = &cobra.Command{
	Use:    "create-branches <source ref uri>",
	Short:  "Create a lot of branches very quickly.",
	Hidden: false,
	Args: cmdutils.ValidationChain(
		cobra.ExactArgs(1),
		cmdutils.FuncValidator(0, uri.ValidateRefURI),
	),
	Run: func(cmd *cobra.Command, args []string) {
		u := uri.Must(uri.Parse(args[0]))

		cleanOnly := MustBool(cmd.Flags().GetBool("clean-only"))
		branchPrefix := MustString(cmd.Flags().GetString("branch-prefix"))
		amount := MustInt(cmd.Flags().GetInt("amount"))
		parallelism := MustInt(cmd.Flags().GetInt("parallelism"))

		deleteGen := stress.NewGenerator(parallelism)

		const paginationAmount = 1000
		deleteGen.Setup(func(add stress.GeneratorAddFn) {
			client := getClient()
			currentOffset := api.PaginationAfter(branchPrefix)
			amount := api.PaginationAmount(paginationAmount)
			for {
				res, err := client.ListBranchesWithResponse(cmd.Context(), u.Repository, &api.ListBranchesParams{
					After:  &currentOffset,
					Amount: &amount,
				})
				DieOnResponseError(res, err)

				for _, ref := range res.JSON200.Results {
					if !strings.HasPrefix(ref.Id, branchPrefix) {
						return
					}
					add(ref.Id) // this branch should be deleted!
				}
				pagination := res.JSON200.Pagination
				if !pagination.HasMore {
					return
				}
				currentOffset = api.PaginationAfter(pagination.NextOffset)
			}
		})

		// wait for deletes to end
		deleteGen.Run(func(input chan string, output chan stress.Result) {
			client := getClient()
			for branch := range input {
				start := time.Now()
				_, err := client.DeleteBranchWithResponse(cmd.Context(), u.Repository, branch)
				output <- stress.Result{
					Error: err,
					Took:  time.Since(start),
				}
			}
		})

		if cleanOnly {
			return // done.
		}

		// start creating branches
		generator := stress.NewGenerator(parallelism, stress.WithSignalHandlersFor(os.Interrupt, syscall.SIGTERM))

		// generate create branch requests
		generator.Setup(func(add stress.GeneratorAddFn) {
			for i := 0; i < amount; i++ {
				add(fmt.Sprintf("%s-%d", branchPrefix, i))
			}
		})

		generator.Run(func(input chan string, output chan stress.Result) {
			client := getClient()
			ctx := cmd.Context()
			for branch := range input {
				start := time.Now()
				resp, err := client.CreateBranchWithResponse(
					ctx, u.Repository, api.CreateBranchJSONRequestBody{
						Name:   branch,
						Source: u.Ref,
					})
				if err == nil && resp.StatusCode() != http.StatusCreated {
					err = fmt.Errorf("%w: %s (%d)", ErrRequestFailed, resp.Status(), resp.StatusCode())
				}
				output <- stress.Result{
					Error: err,
					Took:  time.Since(start),
				}
			}
		})
	},
}

//nolint:gochecknoinits
func init() {
	rootCmd.AddCommand(abuseCmd)

	abuseCmd.AddCommand(abuseCreateBranchesCmd)
	abuseCreateBranchesCmd.Flags().String("branch-prefix", "abuse-", "prefix to create branches under")
	abuseCreateBranchesCmd.Flags().Bool("clean-only", false, "only clean up past runs")
	abuseCreateBranchesCmd.Flags().Int("amount", 1000000, "amount of things to do")
	abuseCreateBranchesCmd.Flags().Int("parallelism", 100, "amount of things to do in parallel")

	abuseCmd.AddCommand(abuseRandomReadsCmd)
	abuseRandomReadsCmd.Flags().String("from-file", "", "read keys from this file (\"-\" for stdin)")
	abuseRandomReadsCmd.Flags().Int("amount", 1000000, "amount of reads to do")
	abuseRandomReadsCmd.Flags().Int("parallelism", 100, "amount of reads to do in parallel")

	abuseCmd.AddCommand(abuseRandomWritesCmd)
	abuseRandomWritesCmd.Flags().String("prefix", "abuse/", "prefix to create paths under")
	abuseRandomWritesCmd.Flags().Int("amount", 1000000, "amount of writes to do")
	abuseRandomWritesCmd.Flags().Int("parallelism", 100, "amount of writes to do in parallel")
}
