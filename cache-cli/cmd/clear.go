package cmd

import (
	"strings"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/files"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/storage"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var clearCmd = &cobra.Command{
	Use:   "clear [branch]",
	Short: "Remove all keys in the cache.",
	Long:  ``,
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		RunClear(cmd, args)
	},
}

func RunClear(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		log.Errorf("Incorrect number of arguments!")
		_ = cmd.Help()
		return
	}

	storage, err := storage.InitStorage()
	utils.Check(err)

	if args[0] == "branch" {
		clearBranchKeys(storage)
	} else {
		err = storage.Clear()
		utils.Check(err)
		log.Infof("Deleted all caches.")
	}
}

func clearBranchKeys(storage storage.Storage) {
	lookupResults := files.Lookup(files.LookupOptions{
		GitBranch: FindGitBranch(),
		Restore:   true,
	})

	if len(lookupResults) == 0 {
		log.Info("Cache is empty.")
		return
	}

	for _, lookupResult := range lookupResults {
		log.Infof("Detected %s.", lookupResult.DetectedFile)
		for _, entry := range lookupResult.Entries {
			log.Infof("Fetching '%s' directory with cache keys '%s'...", entry.Path, strings.Join(entry.Keys, ","))
			findAndDeleteKey(storage, entry.Keys)
		}
	}
}

func findAndDeleteKey(storage storage.Storage, keys []string) {
	for _, rawKey := range keys {
		key := NormalizeKey(rawKey)
		if ok, _ := storage.HasKey(key); ok {
			err := storage.Delete(key)
			utils.Check(err)
			log.Infof("Key '%s' is deleted.", key)
		} else {
			log.Infof("Key '%s' doesn't exist in the cache store.", key)
		}
	}
}

func init() {
	RootCmd.AddCommand(clearCmd)
}
