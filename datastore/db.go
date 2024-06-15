package datastore

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const outFileName = "current-data"

var ErrNotFound = fmt.Errorf("record does not exist")


const bufferSize = 8192


type hashIndex map[string]int64
// ok

type indexOperation struct {
	isWrite bool
	key     string
	offset  int64
}


type KeyPosition struct {
	segment  *Segment
	offset int64
}

type Segment struct {
	outOffset int64

	index    hashIndex
	filePath string
}

type Db struct {
	out              *os.File
	outPath          string
	outOffset        int64
	dir              string

	segmentSizeBytes int64
	lastSegmentIndex int
	indexOperations  chan indexOperation
	positionLookups  chan *KeyPosition
	putOperations    chan entry
	putFinished      chan error
	index    hashIndex
	segments []*Segment
}


func NewDb(dir string) (*Db, error) {
	outputPath := filepath.Join(dir, outFileName)
	f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	db := &Db{
		outPath: outputPath,
		out:     f,
		index:   make(hashIndex),
	}
	err = db.recover()
	if err != nil && err != io.EOF {
		return nil, err
	}
	return db, nil
}


func (db *Db) recover() error {
	input, err := os.Open(db.outPath)
	if err != nil {
		return err
	}
	defer input.Close()

	var buf [bufferSize]byte
	in := bufio.NewReaderSize(input, bufferSize)
	for err == nil {
		var (
			header, data []byte
			n int
		)
		header, err = in.Peek(bufferSize)
		if err == io.EOF {
			if len(header) == 0 {
				return err
			}
		} else if err != nil {
			return err
		}
		size := binary.LittleEndian.Uint32(header)

		if size < bufferSize {
			data = buf[:size]
		} else {
			data = make([]byte, size)
		}
		n, err = in.Read(data)

		if err == nil {
			if n != int(size) {
				return fmt.Errorf("corrupted file")
			}

			var e entry
			e.Decode(data)
			db.index[e.key] = db.outOffset
			db.outOffset += int64(n)
		}
	}
	return err
}

func (db *Db) Close() error {
	return db.out.Close()
}

func (db *Db) Get(key string) (string, error) {
	position, ok := db.index[key]
	if !ok {
		return "", ErrNotFound
	}

	file, err := os.Open(db.outPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = file.Seek(position, 0)
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(file)
	value, err := readValue(reader)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (db *Db) Put(key, value string) error {
	e := entry{
		key:   key,
		value: value,
	}
	n, err := db.out.Write(e.Encode())
	if err == nil {
		db.index[key] = db.outOffset
		db.outOffset += int64(n)
	}
	return err
}


func (db *Db) createNewSegment() error {
	segmentFileName := db.generateSegmentFileName()
	segmentFile, err := os.OpenFile(segmentFileName, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		return err
	}

	newSegment := &Segment{
		filePath: segmentFileName,
		index:    make(hashIndex),
	}
	db.out = segmentFile
	db.outOffset = 0
	db.segments = append(db.segments, newSegment)
	const segmentsLength = 3;
	// if len(db.segments) >= segmentsLength {
	// 	db.compactAndMergeSegments()
	// }
	return err
}

func (db *Db) generateSegmentFileName() string {
	segmentFileName := filepath.Join(db.dir, fmt.Sprintf("%s%d", outFileName, db.lastSegmentIndex))
	db.lastSegmentIndex++
	return segmentFileName
}


func (db *Db) mergeSegments() {
	newSegmentFileName := db.generateSegmentFileName()
	newSegment := &Segment{
		filePath: newSegmentFileName,
		index:    make(hashIndex),
	}
	var offset int64
	newSegmentFile, err := os.OpenFile(newSegmentFileName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return
	}
	lastSegmentIndex := len(db.segments) - 2
	for i := 0; i <= lastSegmentIndex; i++ {
		segment := db.segments[i]
		for key, index := range segment.index {
			if i < lastSegmentIndex {
				isInNewerSegments := hasKeyInSegments(db.segments[i+1:lastSegmentIndex+1], key)
				if isInNewerSegments {
					continue
				}
			}
			value, _ := segment.fetchValueFromSegment(index)
			entry := entry{
				key:   key,
				value: value,
			}
			n, err := newSegmentFile.Write(entry.Encode())
			if err == nil {
				newSegment.index[key] = offset
				offset += int64(n)
			}
		}
	}
	db.segments = []*Segment{newSegment, db.getCurrentSegment()}
}


func hasKeyInSegments(segments []*Segment, keyToFind string) bool {
	for _, segment := range segments {
		if _, keyExists := segment.index[keyToFind]; keyExists {
			return true
		}
	}
	return false
}

func (segment *Segment) fetchValueFromSegment(offset int64) (string, error) {
	segmentFile, err := os.Open(segment.filePath)
	if err != nil {
		return "", err
	}
	defer segmentFile.Close()

	_, err = segmentFile.Seek(offset, 0)
	if err != nil {
		return "", err
	}

	segmentReader := bufio.NewReader(segmentFile)
	value, err := readValue(segmentReader)
	if err != nil {
		return "", err
	}
	return value, nil
}


func (db *Db) getCurrentSegment() *Segment {
	lastSegmentIndex := len(db.segments) - 1
	return db.segments[lastSegmentIndex]
}
