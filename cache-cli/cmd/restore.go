package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"regexp"
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

var restoreCmd = &cobra.Command{
	Use:   "restore [keys]",
	Short: "Restore keys from the cache.",
	Long:  ``,
	Args:  cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		RunRestore(cmd, args)
	},
}

func RunRestore(cmd *cobra.Command, args []string) {
	if len(args) > 1 {
		log.Error("Incorrect number of arguments!")
		_ = cmd.Help()
		return
	}

	storage, err := storage.InitStorage()
	utils.Check(err)

	metricsManager, err := metrics.InitMetricsManager(metrics.LocalBackend)
	utils.Check(err)

	archiver := archive.NewArchiver(metricsManager)

	if len(args) == 0 {
		lookupResults := files.Lookup(files.LookupOptions{
			RootDirectory: FindGitRoot(),
			GitBranch:     FindGitBranch(),
			DefaultBranch: os.Getenv("NEETO_CI_DEFAULT_BRANCH"),
			Restore:       true,
		})

		if len(lookupResults) == 0 {
			log.Info("Nothing to restore from cache.")
			return
		}

		// nvm, gems, node-modules etc. are independent, so restore them
		// concurrently instead of one cache key after another.
		var wg sync.WaitGroup
		for _, lookupResult := range lookupResults {
			log.Infof("Detected %s.", lookupResult.DetectedFile)
			for _, entry := range lookupResult.Entries {
				wg.Add(1)
				go func(entry files.LookupResultEntry) {
					defer wg.Done()
					log.Infof("Fetching '%s' directory with cache keys '%s'...", entry.Path, strings.Join(entry.Keys, ","))
					downloadAndUnpack(storage, archiver, metricsManager, entry.Keys, findAutomaticKey)
				}(entry)
			}
		}
		wg.Wait()
	} else {
		keys := strings.Split(args[0], ",")
		downloadAndUnpack(storage, archiver, metricsManager, keys, findMatchingKey)
	}
}

type keyMatcher func(availableKeys []storage.CacheKey, match string) string

func downloadAndUnpack(storage storage.Storage, archiver archive.Archiver, metricsManager metrics.MetricsManager, keys []string, findKey keyMatcher) {
	for _, rawKey := range keys {
		key := NormalizeKey(rawKey)
		if ok, _ := storage.HasKey(key); ok {
			log.Infof("HIT: '%s', using key '%s'.", key, key)
			downloadAndUnpackKey(storage, archiver, metricsManager, key)
			break
		}

		availableKeys, err := storage.List()
		utils.Check(err)

		matchingKey := findKey(availableKeys, key)
		if matchingKey != "" {
			log.Infof("HIT: '%s', using key '%s'.", key, matchingKey)
			downloadAndUnpackKey(storage, archiver, metricsManager, matchingKey)
			break
		} else {
			log.Infof("MISS: '%s'.", key)
		}
	}
}

func findMatchingKey(availableKeys []storage.CacheKey, match string) string {
	for _, availableKey := range availableKeys {
		isMatch, _ := regexp.MatchString(match, availableKey.Name)
		if isMatch {
			return availableKey.Name
		}
	}

	return ""
}

func findAutomaticKey(availableKeys []storage.CacheKey, fallbackKey string) string {
	for _, availableKey := range availableKeys {
		if files.IsAutomaticKeyFor(fallbackKey, availableKey.Name) {
			return availableKey.Name
		}
	}

	return ""
}

func downloadAndUnpackKey(storage storage.Storage, archiver archive.Archiver, metricsManager metrics.MetricsManager, key string) {
	downloadStart := time.Now()
	log.Infof("Downloading key '%s'...", key)
	compressed, err := storage.Restore(key)
	utils.Check(err)

	downloadDuration := time.Since(downloadStart)
	info, _ := os.Stat(compressed.Name())

	log.Infof("Download complete. Duration: %v. Size: %v bytes.", downloadDuration.String(), files.HumanReadableSize(info.Size()))
	publishMetrics(metricsManager, info, downloadDuration)

	unpackStart := time.Now()
	log.Infof("Unpacking '%s'...", compressed.Name())
	restorationPath, err := archiver.Decompress(compressed.Name())
	utils.Check(err)

	unpackDuration := time.Since(unpackStart)
	log.Infof("Unpack complete. Duration: %v.", unpackDuration)
	log.Infof("Restored: %s.", restorationPath)

	err = os.Remove(compressed.Name())
	if err != nil {
		log.Errorf("Error removing %s: %v", compressed.Name(), err)
	}
}

func publishMetrics(metricsManager metrics.MetricsManager, fileInfo fs.FileInfo, downloadDuration time.Duration) {
	metricsToPublish := []metrics.Metric{
		{Name: metrics.CacheDownloadSize, Value: fmt.Sprintf("%d", fileInfo.Size())},
		{Name: metrics.CacheDownloadTime, Value: downloadDuration.String()},
	}

	username := os.Getenv("NEETO_CI_CACHE_USERNAME")
	if username != "" {
		metricsToPublish = append(metricsToPublish, metrics.Metric{Name: metrics.CacheUser, Value: username})
	}

	cacheServerIP := getCacheServerIP()
	if cacheServerIP != "" {
		metricsToPublish = append(metricsToPublish, metrics.Metric{Name: metrics.CacheServer, Value: cacheServerIP})
	}

	metricsToPublish = append(metricsToPublish, metrics.Metric{Name: metrics.CacheTotalRate, Value: "1"})

	err := metricsManager.PublishBatch(metricsToPublish)
	if err != nil {
		log.Errorf("Error publishing metrics: %v", err)
	}
}

func getCacheServerIP() string {
	cacheURL := os.Getenv("NEETO_CI_CACHE_URL")
	if cacheURL != "" {
		ipAndPort := strings.Split(cacheURL, ":")
		if len(ipAndPort) != 2 {
			return ""
		}

		return ipAndPort[0]
	}

	return ""
}

func init() {
	RootCmd.AddCommand(restoreCmd)
}
