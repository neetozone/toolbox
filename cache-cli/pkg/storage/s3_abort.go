package storage

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	log "github.com/sirupsen/logrus"
)

const abortTimeout = 30 * time.Second

// uploadContext is cancelled by SIGINT or SIGTERM; stop returns the signal received, if any.
var uploadContext = func() (context.Context, func() os.Signal) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)

	var received os.Signal
	done := make(chan struct{})
	go func() {
		defer close(done)
		select {
		case received = <-signals:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, func() os.Signal {
		signal.Stop(signals)
		cancel()
		<-done
		return received
	}
}

// exitProcess is os.Exit, replaceable in tests.
var exitProcess = os.Exit

// exitOnSignal ends the process as the signal would have, after the upload cleaned up.
func exitOnSignal(sig os.Signal) {
	log.Errorf("Interrupted by %v.", sig)
	code := 1
	if number, ok := sig.(syscall.Signal); ok {
		code = 128 + int(number)
	}

	exitProcess(code)
}

// abortInterruptedUpload discards the parts of a multipart upload stopped by a signal.
// The SDK aborts other failed uploads itself, but its abort reuses the cancelled context.
func (s *S3Storage) abortInterruptedUpload(key string, uploadErr error) {
	var failure manager.MultiUploadFailure
	if !errors.As(uploadErr, &failure) {
		return
	}

	abortCtx, cancel := context.WithTimeout(context.Background(), abortTimeout)
	defer cancel()

	uploadID := failure.UploadID()
	_, err := s.Client.AbortMultipartUpload(abortCtx, &s3.AbortMultipartUploadInput{
		Bucket:   &s.Bucket,
		Key:      &key,
		UploadId: &uploadID,
	})

	if err != nil {
		log.Errorf("Error aborting interrupted upload of '%s': %v", key, err)
		return
	}

	log.Infof("Aborted interrupted upload of '%s'.", key)
}
