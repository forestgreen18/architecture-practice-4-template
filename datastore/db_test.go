package datastore

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDb_Put(t *testing.T) {


	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new instance of the database in the temporary directory
	dbInstance, err := NewDb(tempDir, 150)
	if err != nil {
		t.Fatal(err)
	}
	defer dbInstance.Close()

	// Create a slice of key-value keyValuePairs to be inserted into the database
	keyValuePairs := [][]string{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	// Open the output file associated with the first segment
	outputFilePath := filepath.Join(tempDir, outFileName+"0")
	outputFile, err := os.Open(outputFilePath)
	if err != nil {
		t.Fatal(err)
	}



	t.Run("put/get", func(t *testing.T) {
		for _, kvPair := range keyValuePairs {
			key, value := kvPair[0], kvPair[1]
			insertErr := dbInstance.Put(key, value)
			if insertErr != nil {
				t.Errorf("Unable to insert key %s: %s", key, insertErr)
			}
			storedValue, getErr := dbInstance.Get(key)
			if getErr != nil {
				t.Errorf("Unexpected error when getting %s: %s", key, getErr)
			}
			if storedValue != value {
				t.Errorf("Unexpected value for key %s, expected %s, got %s", key, value, storedValue)
			}
		}
	})


	fileInfo, err := outputFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	initialFileSize := fileInfo.Size()


	t.Run("file growth", func(t *testing.T) {
		for _, keyValuePair := range keyValuePairs {
			err := dbInstance.Put(keyValuePair[0], keyValuePair[1])
			if err != nil {
				t.Errorf("Failed to put %s: %s", keyValuePair[0], err)
			}
		}
		fileInfo, err := outputFile.Stat()
		if err != nil {
			t.Fatal(err)
		}
		expectedFileSize := initialFileSize * 2
		if expectedFileSize != fileInfo.Size() {
			t.Errorf("Unexpected file size (expected %d, got %d)", expectedFileSize, fileInfo.Size())
		}
	})
	t.Run("new db process", func(t *testing.T) {
		if closeErr := dbInstance.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		newDb, creationErr := NewDb(tempDir, 100)
		if creationErr != nil {
			t.Fatal(creationErr)
		}

		for _, keyValuePair := range keyValuePairs {
			fetchedValue, fetchErr := newDb.Get(keyValuePair[0])
			if fetchErr != nil {
				t.Errorf("Unable to fetch %s: %s", keyValuePair[0], fetchErr)

			}
			if fetchedValue != keyValuePair[1] {
				t.Errorf("Unexpected value for %s, expected %s, got %s", keyValuePair[0], keyValuePair[1], fetchedValue)
			}
		}
	})
}

func TestDb_Segmentation(t *testing.T) {

	// Create a temporary directory for the test database
	tempDir, err := os.MkdirTemp("", "test-db")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create a new instance of the database in the temporary directory
	dbInstance, err := NewDb(tempDir, 45)
	if err != nil {
		t.Fatal(err)
	}
	defer dbInstance.Close()

	t.Run("should create two files", func(t *testing.T) {
		dbInstance.Put("key1", "value1")
		dbInstance.Put("key2", "value2")
		dbInstance.Put("key3", "value3")
		dbInstance.Put("key2", "value5")

		if len(dbInstance.segments) != 2 {
			t.Errorf("Expected 2 files, got %d", len(dbInstance.segments))
		}
	})



	t.Run("should remove segment after time", func(t *testing.T) {
		dbInstance.Put("key4", "value4")

		segmentCount := len(dbInstance.segments)
		if segmentCount != 3 {
			t.Errorf("Expected 3 segments, got %d", segmentCount)
		}

		time.Sleep(2 * time.Second)

		segmentCount = len(dbInstance.segments)
		if segmentCount != 2 {
			t.Errorf("Expected 2 segments, got %d", segmentCount)
		}
	})


	t.Run("shouldn't store duplicate key values", func(t *testing.T) {
		fileInfo, err := os.Stat(dbInstance.segments[0].filePath)
		if err != nil {
			t.Fatal(err)
		}

		expectedSize := int64(66)
		if fileInfo.Size() != expectedSize {
			t.Errorf("Expected file size %d, but got %d", expectedSize, fileInfo.Size())
		}
	})

	t.Run("should keep the last value of a duplicate key", func(t *testing.T) {
		lastValue, err := dbInstance.Get("key2")
		if err != nil {
			t.Errorf("Failed to get value of key 'key2'. Error: %s", err)
		}
		if lastValue != "value5" {
			t.Errorf("Expected value 'value5' for key 'key2', but got '%s'", lastValue)
		}
	})
}
