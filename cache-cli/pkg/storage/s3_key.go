package storage

import "fmt"

// MaxS3ObjectKeyBytes is S3's limit on the UTF-8 length of an object key.
const MaxS3ObjectKeyBytes = 1024

func (s *S3Storage) objectKey(key string) (string, error) {
	objectKey := fmt.Sprintf("%s/%s", s.Project, key)
	if len(objectKey) > MaxS3ObjectKeyBytes {
		return "", fmt.Errorf(
			"cache key is too long: the S3 object key '%s/<key>' is %d bytes and S3 allows at most %d; use a key of at most %d bytes",
			s.Project, len(objectKey), MaxS3ObjectKeyBytes, MaxS3ObjectKeyBytes-len(s.Project)-1,
		)
	}

	return objectKey, nil
}
