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
	"google.golang.org/api/option"
)

var ErrObjectNotExist = storage.ErrObjectNotExist

type GStorage struct {
	storageClient *storage.Client
	objectsPrefix string
}

func NewGStorage(credentialsFilePath, objectsPrefix string) (*GStorage, error) {
	var client *storage.Client
	var err error

	if credentialsFilePath != "" {
		client, err = storage.NewClient(context.Background(), option.WithCredentialsFile(credentialsFilePath))
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

	return &GStorage{storageClient: client, objectsPrefix: objectsPrefix}, nil
}

// UploadFile uploads an object.
func (gs *GStorage) UploadFile(bucket, filePath string) error {

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
	wc := gs.storageClient.Bucket(bucket).Object(object).NewWriter(ctx)
	if _, err = io.Copy(wc, f); err != nil {
		return fmt.Errorf("io.Copy: %v", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("Writer.Close: %v", err)
	}

	fmt.Printf("Blob %v uploaded to %v.\n", filepath.Base(filePath), object)
	return nil
}

// DownloadFile downloads an object to a file.
func (gs *GStorage) DownloadFile(bucket, object string, destFileName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*50)
	defer cancel()

	// TODO: Look at updating the permissions for this
	f, err := os.Create(destFileName)
	if err != nil {
		return fmt.Errorf("os.Create: %v", err)
	}

	rc, err := gs.storageClient.Bucket(bucket).Object(gs.objectsPrefix + object).NewReader(ctx)
	if err == storage.ErrObjectNotExist {
		return err
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

	fmt.Printf("Blob %v downloaded to local file %v\n", object, destFileName)

	return nil

}
