package cmd

import (
	"os"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/storage"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// has_key exit codes: an error must never look like "the key exists".
const (
	HasKeyExists  = 0
	HasKeyMissing = 1
	HasKeyUnknown = 2
)

var hasKeyCmd = &cobra.Command{
	Use:   "has_key [key]",
	Short: "Check if a key is present in the cache.",
	Long: `Check if a key is present in the cache.

Exits 0 when the key exists, 1 when it does not, and 2 when that could not
be determined, for example because the cache backend is unreachable.`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if code := RunHasKey(cmd, args); code != HasKeyExists {
			os.Exit(code)
		}
	},
}

func RunHasKey(cmd *cobra.Command, args []string) int {
	if len(args) != 1 {
		log.Error("Incorrect number of arguments!")
		_ = cmd.Help()
		return HasKeyUnknown
	}

	storage, err := storage.InitStorage()
	if err != nil {
		log.Errorf("error: %s", err.Error())
		return HasKeyUnknown
	}

	rawKey := args[0]
	key := NormalizeKey(rawKey)
	exists, err := storage.HasKey(key)
	if err != nil {
		log.Errorf("error: could not check key '%s': %s", key, err.Error())
		return HasKeyUnknown
	}

	if exists {
		log.Infof("Key '%s' exists in the cache store.", key)
		return HasKeyExists
	}

	log.Infof("Key '%s' doesn't exist in the cache store.", key)
	return HasKeyMissing
}

func init() {
	RootCmd.AddCommand(hasKeyCmd)
}
