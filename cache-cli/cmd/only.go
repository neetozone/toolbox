package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/files"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const onlyFlag = "only"

func addOnlyFlag(cmd *cobra.Command) {
	cmd.Flags().String(onlyFlag, "", fmt.Sprintf(
		"In automatic mode, only process these comma-separated types: %s.",
		strings.Join(files.EntryTypes(), ", "),
	))
}

// onlyTypes returns nil when --only is not given.
func onlyTypes(cmd *cobra.Command, args []string) ([]string, error) {
	flag := cmd.Flags().Lookup(onlyFlag)
	if flag == nil || !flag.Changed {
		return nil, nil
	}

	if len(args) > 0 {
		return nil, errors.New("--only works only in automatic mode, not with explicit keys")
	}

	return files.ParseEntryTypes(flag.Value.String())
}

// exitOnUsageError exits 1, unlike cache errors, which honour CACHE_FAIL_ON_ERROR.
func exitOnUsageError(cmd *cobra.Command, err error) {
	if err == nil {
		return
	}

	log.Errorf("error: %v", err)
	_ = cmd.Help()
	os.Exit(1)
}

func lookupAutomatic(options files.LookupOptions, types []string) []files.LookupResult {
	if types == nil {
		return files.Lookup(options)
	}

	lookup := files.LookupTypesDetailed(options, types)
	for _, entryType := range lookup.Missing {
		log.Infof("Skipping %s cache: no lock file for it in this directory.", entryType)
	}

	for _, entryType := range lookup.Unreadable {
		log.Warnf("Skipping %s cache: its lock file exists but could not be read.", entryType)
	}

	return lookup.Results
}
