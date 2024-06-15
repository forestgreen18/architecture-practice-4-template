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


func NewDb(dir string, segmentSizeBytes int64) (*Db, error) {
	db := &Db{
		segments:     make([]*Segment, 0),
		dir:          dir,
		segmentSizeBytes:  segmentSizeBytes,
		indexOperations:     make(chan indexOperation),
		positionLookups: make(chan *KeyPosition),
		putOperations:       make(chan entry),
		putFinished:      make(chan error),
	}

	err := db.createNewSegment()
	if err != nil {
		return nil, err
	}

	err = db.recoverData()
	if err != nil && err != io.EOF {
		return nil, err
	}

	db.startRoutineForIndexOps()
	db.startPutRoutine()

	return db, nil
}


func (db *Db) startRoutineForIndexOps() {
	processIndexOp := func(op indexOperation) {
		if op.isWrite {
			db.updateOffset(op.key, op.offset)
		} else {
			segment, position, err := db.locateKey(op.key)
			if err != nil {
				db.positionLookups <- nil
			} else {
				db.positionLookups <- &KeyPosition{
					segment,
					position,
				}
			}
		}
	}

	go func() {
		for op := range db.indexOperations {
			processIndexOp(op)
		}
	}()
}


// okay.
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
	if len(db.segments) >= 3 {
		db.compactAndMergeSegments()
	}
	return err
}

func (db *Db) generateSegmentFileName() string {
	segmentFileName := filepath.Join(db.dir, fmt.Sprintf("%s%d", outFileName, db.lastSegmentIndex))
	db.lastSegmentIndex++
	return segmentFileName
}
// done

func (db *Db) compactAndMergeSegments() {
	go db.mergeSegments()
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

func (db *Db) recoverData() error {
	var err error
	buf := make([]byte, bufferSize)

	reader := bufio.NewReaderSize(db.out, bufferSize)
	for err == nil {
		var (
			header []byte
			data   []byte
			n      int
		)
		header, err = reader.Peek(bufferSize)
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
		n, err = reader.Read(data)

		if err == nil {
			if n != int(size) {
				return fmt.Errorf("corrupted file")
			}

			entry := entry{}
			entry.Decode(data)
			db.updateOffset(entry.key, int64(n))
		}
	}
	return err
}
func (db *Db) Close() error {
	return db.out.Close()
}

func (db *Db) updateOffset(key string, dataSize int64) {
	lastSegment := db.getCurrentSegment()
	lastSegment.index[key] = db.outOffset
	db.outOffset += dataSize
}
func (db *Db) locateKey(searchKey string) (*Segment, int64, error) {
	for segmentIndex := len(db.segments) - 1; segmentIndex >= 0; segmentIndex-- {
		currentSegment := db.segments[segmentIndex]
		position, keyExists := currentSegment.index[searchKey]
		if keyExists {
			return currentSegment, position, nil
		}
	}

	return nil, 0, ErrNotFound
}

func (db *Db) fetchKeyPosition(searchKey string) *KeyPosition {
	indexOperation := indexOperation{
		isWrite: false,
		key:     searchKey,
	}
	db.indexOperations <- indexOperation
	return <-db.positionLookups
}

func (db *Db) Get(key string) (string, error) {
	keyPos := db.fetchKeyPosition(key)
	if keyPos == nil {
		return "", ErrNotFound
	}
	value, err := keyPos.segment.fetchValueFromSegment(keyPos.offset)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (db *Db) getCurrentSegment() *Segment {
	lastSegmentIndex := len(db.segments) - 1
	return db.segments[lastSegmentIndex]
}
func (db *Db) startPutRoutine() {
	go func() {
		for {
			entry := <-db.putOperations
			length := entry.getLength()
			fileStat, err := db.out.Stat()
			if err != nil {
				db.putFinished <- err
				continue
			}
			if fileStat.Size()+length > db.segmentSizeBytes {
				err := db.createNewSegment()
				if err != nil {
					db.putFinished <- err
					continue
				}
			}
			n, err := db.out.Write(entry.Encode())
			if err == nil {
				db.indexOperations <- indexOperation{
					isWrite: true,
					key:     entry.key,
					offset:   int64(n),
				}
			}
			db.putFinished <- nil
		}
	}()
}
func (db *Db) Put(key, value string) error {
	e := entry{
		key:   key,
		value: value,
	}
	db.putOperations <- e
	return <-db.putFinished
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
