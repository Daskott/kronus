package gstorage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/Daskott/kronus/colors"
	"github.com/Daskott/kronus/server/logger"
	"google.golang.org/api/option"
)

var (
	ErrObjectNotExist = storage.ErrObjectNotExist
	logg              = logger.NewLogger()
)

type GStorage struct {
	storageClient *storage.Client
	bucket        string
	objectsPrefix string
}

func NewGStorage(credentialsFilePath, bucket, objectsPrefix string) (*GStorage, error) {
	var client *storage.Client
	var err error

	if credentialsFilePath != "" {
		client, err = storage.NewClient(
			context.Background(),
			option.WithCredentialsFile(credentialsFilePath))
	} else {
		client, err = storage.NewClient(context.Background())
	}

	if err != nil {
		return nil, fmt.Errorf("NewGStorage: %v", err)
	}

	// Add slash to 'objectsPrefix' if non, to act as folder in gstorage
	if !strings.HasSuffix(objectsPrefix, "/") {
		objectsPrefix += "/"
	}

	return &GStorage{
		storageClient: client,
		bucket:        bucket,
		objectsPrefix: objectsPrefix,
	}, nil
}

// UploadFile uploads an object.
func (gs *GStorage) UploadFile(filePath string) error {

	// Open local file in filePath
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("os.Open: %v", err)
	}
	defer f.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
	defer cancel()

	// Upload an object with storage.Writer.
	object := gs.objectsPrefix + filepath.Base(filePath)
	wc := gs.storageClient.Bucket(gs.bucket).Object(object).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}

	gs.logInfof("%v uploaded to '%v'", filepath.Base(filePath), object)
	return nil
}

// DownloadFile downloads an object to a file.
func (gs *GStorage) DownloadFile(object string, destFileName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
	defer cancel()

	rc, err := gs.storageClient.Bucket(gs.bucket).Object(gs.objectsPrefix + object).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return err
	}

	f, err := os.Create(destFileName)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}

	if err != nil {
		return fmt.Errorf("Object(%q).NewReader: %v", object, err)
	}
	defer rc.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("f.Close: %v", err)
	}

	gs.logInfof("'%v' downloaded to local file %v", object, destFileName)

	return nil

}

func (gs *GStorage) logInfof(template string, args ...interface{}) {
	prefix := colors.Green("[gstorage] ")
	logg.Infof(prefix+template, args...)
}
