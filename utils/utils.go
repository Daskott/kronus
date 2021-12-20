package utils

import (
	"log"
	"os"
)

func FileExist(filePath string) bool {
	var err error

	if _, err = os.Stat(filePath); os.IsNotExist(err) {
		return false
	}

	if err != nil {
		log.Panic(err)
	}

	return true
}

func CreateDirIfNotExist(dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		err := os.Mkdir(dir, 0755)
		if err != nil {
			return err
		}
	}

	return nil
}
