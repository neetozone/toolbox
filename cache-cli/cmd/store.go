package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/archive"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/files"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/metrics"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/storage"
	"github.com/semaphoreci/toolbox/cache-cli/pkg/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func NewStoreCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "store [key path]",
		Short: "Store keys in the cache.",
		Long:  ``,
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			RunStore(cmd, args)
		},
	}

	description := fmt.Sprintf(`
		If storage does not have enough space,
		keys will be sorted in descending order using the specified field,
		and will be cleaned up starting from the last key on the list.
		Possible values are: %v.
	`, strings.Join(storage.ValidSortByKeys, ","))

	cmd.Flags().StringP("cleanup-by", "c", storage.SortByStoreTime, description)
	addOnlyFlag(cmd)
	return cmd
}

func RunStore(cmd *cobra.Command, args []string) {
	if len(args) != 0 && len(args) != 2 {
		log.Error("Incorrect number of arguments!")
		_ = cmd.Help()
		return
	}

	types, err := onlyTypes(cmd, args)
	exitOnUsageError(cmd, err)

	cleanupBy, err := cmd.Flags().GetString("cleanup-by")
	utils.Check(err)

	storage, err := storage.InitStorageWithConfig(storage.StorageConfig{SortKeysBy: cleanupBy})
	utils.Check(err)

	metricsManager, err := metrics.InitMetricsManager(metrics.LocalBackend)
	utils.Check(err)

	archiver := archive.NewArchiver(metricsManager)

	if len(args) == 0 {
		gitBranch := FindGitBranch()
		if gitBranch == "" {
			log.Info("Skipping automatic store: could not determine the git branch.")
			return
		}

		lookupResults := lookupAutomatic(files.LookupOptions{
			RootDirectory: FindGitRoot(),
			GitBranch:     gitBranch,
			DefaultBranch: os.Getenv("NEETO_CI_DEFAULT_BRANCH"),
			Restore:       false,
		}, types)

		if len(lookupResults) == 0 {
			log.Info("Nothing to store in cache.")
			return
		}

		// Each entry's compress + upload is independent, so store them
		// concurrently instead of one cache key after another.
		var wg sync.WaitGroup
		for _, lookupResult := range lookupResults {
			log.Infof("Detected %s.", lookupResult.DetectedFile)
			for _, entry := range lookupResult.Entries {
				wg.Add(1)
				go func(entry files.LookupResultEntry) {
					defer wg.Done()
					log.Infof("Using default cache path '%s'.", entry.Path)
					compressAndStore(storage, archiver, entry.Keys[0], entry.Path, entry.EquivalentKeys)
				}(entry)
			}
		}
		wg.Wait()
	} else {
		path := filepath.FromSlash(args[1])
		compressAndStore(storage, archiver, args[0], path, nil)
	}
}

func compressAndStore(storage storage.Storage, archiver archive.Archiver, rawKey, path string, equivalentKeys []string) {
	key := NormalizeKey(rawKey)
	if _, err := os.Stat(path); err == nil {
		if ok, _ := storage.HasKey(key); ok {
			log.Infof("Key '%s' already exists.", key)
			return
		}

		for _, equivalentKey := range equivalentKeys {
			if ok, _ := storage.HasKey(NormalizeKey(equivalentKey)); ok {
				log.Infof("Key '%s' skipped: identical content already cached as '%s'.", key, equivalentKey)
				return
			}
		}

		compressedFilePath, compressedFileSize, err := compress(archiver, key, path)
		if err != nil {
			log.Errorf("Error compressing %s: %v", path, err)
			return
		}

		maxSpace := storage.Config().MaxSpace
		if compressedFileSize > maxSpace {
			log.Errorf("Archive exceeds allocated %s for cache.", files.HumanReadableSize(maxSpace))
			return
		}

		uploadStart := time.Now()
		log.Infof("Uploading '%s' with cache key '%s'...", path, key)
		err = storage.Store(key, compressedFilePath)
		if err != nil {
			_ = os.Remove(compressedFilePath)
			utils.Check(fmt.Errorf("storing key '%s': %w", key, err))
		}

		uploadDuration := time.Since(uploadStart)
		log.Infof("Upload complete. Duration: %v.", uploadDuration)

		err = os.Remove(compressedFilePath)
		if err != nil {
			log.Errorf("Error removing %s: %v", compressedFilePath, err)
		}
	} else {
		log.Infof("'%s' doesn't exist locally.", path)
	}
}

func compress(archiver archive.Archiver, key, path string) (string, int64, error) {
	compressingStart := time.Now()
	log.Infof("Compressing %s...", path)

	dst := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d", utils.TempFilePrefix(key), time.Now().Nanosecond()))
	err := archiver.Compress(dst, path)
	utils.Check(err)

	compressionDuration := time.Since(compressingStart)
	info, err := os.Stat(dst)
	if err != nil {
		_ = os.Remove(dst)
		return "", -1, err
	}

	log.Infof("Compression complete. Duration: %v. Size: %v bytes.", compressionDuration.String(), files.HumanReadableSize(info.Size()))
	return dst, info.Size(), nil
}

func NormalizeKey(key string) string {
	normalizedKey := strings.ReplaceAll(key, "/", "-")
	if normalizedKey != key {
		log.Infof("Key '%s' is normalized to '%s'.", key, normalizedKey)
	}

	return normalizedKey
}

func FindGitBranch() string {
	gitPrBranch := os.Getenv("NEETO_CI_GIT_PR_BRANCH")
	if gitPrBranch != "" {
		return gitPrBranch
	}

	gitBranch := os.Getenv("NEETO_CI_GIT_BRANCH")
	if gitBranch != "" {
		return gitBranch
	}

	// A detached HEAD reports 'HEAD', which names no branch.
	checkedOutBranch := runGit("rev-parse", "--abbrev-ref", "HEAD")
	if checkedOutBranch == "HEAD" {
		return ""
	}

	return checkedOutBranch
}

func FindGitRoot() string {
	return runGit("rev-parse", "--show-toplevel")
}

var gitWarningOnce sync.Once

func runGit(args ...string) string {
	output, err := exec.Command("git", args...).Output()
	if err != nil {
		gitWarningOnce.Do(func() { log.Warn(gitFailureMessage(args, err)) })
		return ""
	}

	return strings.TrimSpace(string(output))
}

func gitFailureMessage(args []string, err error) string {
	reason := err.Error()
	if exitErr, ok := err.(*exec.ExitError); ok {
		if line := strings.TrimSpace(strings.SplitN(string(exitErr.Stderr), "\n", 2)[0]); line != "" {
			reason = line
		}
	}

	message := fmt.Sprintf("git %s failed: %s; automatic cache keys are not scoped by directory", strings.Join(args, " "), reason)
	// Never bypass safe.directory: it guards against attacker-controlled repo config.
	if strings.Contains(reason, "dubious ownership") {
		message += "; make the checkout owned by the user running 'cache'"
	}

	return message
}

func init() {
	RootCmd.AddCommand(NewStoreCommand())
}
