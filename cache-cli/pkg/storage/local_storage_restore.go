package storage

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/semaphoreci/toolbox/cache-cli/pkg/utils"
)

func (s *LocalStorage) Restore(key string) (*os.File, error) {
	localFile, err := ioutil.TempFile(os.TempDir(), fmt.Sprintf("%s-*", utils.TempFilePrefix(key)))
	if err != nil {
		return nil, err
	}

	remoteFilePath := path.Join(s.Path, key)
	remoteFile, err := os.Open(remoteFilePath)
	if err != nil {
		_ = localFile.Close()
		_ = os.Remove(localFile.Name())
		return nil, err
	}

	_, err = localFile.ReadFrom(remoteFile)
	if err != nil {
		_ = localFile.Close()
		_ = remoteFile.Close()
		return nil, err
	}

	err = remoteFile.Close()
	if err != nil {
		_ = localFile.Close()
		return nil, err
	}

	return localFile, localFile.Close()
}
