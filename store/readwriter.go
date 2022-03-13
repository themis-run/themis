package store

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	DefaultLogName       = "themis-log-"
	DefaultIndexFileName = ".index.log" // 存放索引的文件名.
	DefaultDataFileName  = ".data.log"  // 存放数据的文件名.
	IndexHeaderSize      = 24
	HeaderNumSize        = 8
	DataFileNumberShift  = 16
	OffsetShift          = 24
	MaxIndexLogSize      = 4 * 1024 * 1024
	MaxDataLogSize       = 4 * 1024 * 1024
)

var ErrorLogNeedRecover = errors.New("log need recover")
var ErrorDataWrite = errors.New("write data log error")

// ReadWriter do read and write opreations for wal.
type ReadWriter interface {
	Info()

	// next sequenceNumber
	NextSequenceNumber() uint64

	// Write a log, caller should maintain sequenceNumber is incremented.
	Write(sequenceNumber uint64, data []byte) (int, error)

	// ListBefore all data, that sequenceNumber is less than or equal to given sequenceNumber.
	ListBefore(sequenceNumber uint64) ([][]byte, error)
	// ListAfter all data, that sequenceNumber is larger than or equal to given sequenceNumber.
	ListAfter(sequenceNumber uint64) ([][]byte, error)
	// DropBefore all data, that sequenceNumber is less than or equal to given sequenceNumber.
	DropBefore(sequenceNumber uint64) (int, error)

	Close()
}

func NewReadWriter(path string) (ReadWriter, error) {
	err := createDirIfNotExist(path)
	if errors.Is(err, ErrorLogNeedRecover) {
		return recoverReadWriter(path)
	}

	if err != nil {
		return nil, err
	}

	return createReadWriter(path)
}

type readWriterImpl struct {
	path            string
	indexFileNumber uint16
	*indexLogHeader
	index *os.File
	data  *os.File
}

type indexLogHeader struct {
	offset             uint32
	dataFileNumber     uint16
	baseSequenceNumber uint64
	totalKeyNumber     uint64
}

func createReadWriter(path string) (ReadWriter, error) {
	var indexFileNumber uint16 = 0
	var dataFileNumber uint16 = 0

	index, err := openLogFile(path, DefaultIndexFileName, indexFileNumber)
	if err != nil {
		return nil, err
	}

	data, err := openLogFile(path, DefaultDataFileName, dataFileNumber)
	if err != nil {
		return nil, err
	}

	rw := &readWriterImpl{
		path:            path,
		indexFileNumber: indexFileNumber,
		index:           index,
		data:            data,
		indexLogHeader: &indexLogHeader{
			dataFileNumber:     dataFileNumber,
			offset:             0,
			totalKeyNumber:     0,
			baseSequenceNumber: 0,
		},
	}

	if err := rw.initIndexHeader(); err != nil {
		return nil, err
	}

	return rw, nil
}

func recoverReadWriter(path string) (ReadWriter, error) {
	// TODO recover from snap
	indexFileNumber := findIndexLog(path)

	index, err := openLogFile(path, DefaultIndexFileName, indexFileNumber)
	if err != nil {
		return nil, err
	}

	indexHeader, err := readLogHeader(index)
	if err != nil {
		return nil, err
	}

	data, err := openLogFile(path, DefaultDataFileName, indexHeader.dataFileNumber)
	if err != nil {
		return nil, err
	}

	rw := &readWriterImpl{
		path:            path,
		indexFileNumber: indexFileNumber,
		indexLogHeader:  indexHeader,
		index:           index,
		data:            data,
	}
	return rw, nil
}

func readLogHeader(f *os.File) (*indexLogHeader, error) {
	header := make([]byte, IndexHeaderSize)
	_, err := f.Read(header)
	if err != nil {
		return nil, err
	}

	baseSequenceNumber := binary.BigEndian.Uint64(header[:8])
	totalKeyNumber := binary.BigEndian.Uint64(header[8:16])

	indexData := parseIndexData(header[16:])

	return &indexLogHeader{
		offset:             indexData.offset,
		dataFileNumber:     indexData.dataFileNumber,
		baseSequenceNumber: baseSequenceNumber,
		totalKeyNumber:     totalKeyNumber,
	}, nil
}

func findIndexLog(path string) uint16 {
	var indexFileNumber uint16 = 0
	for {
		filename := filepath.Join(path, fmt.Sprintf("%s%d%s", DefaultLogName, indexFileNumber, DefaultIndexFileName))
		f, err := os.Stat(filename)
		if errors.Is(err, os.ErrNotExist) {
			return indexFileNumber
		}

		if f.Size() < MaxIndexLogSize {
			return indexFileNumber
		}

		indexFileNumber++
	}
}

func openLogFile(path, suffix string, num uint16) (*os.File, error) {
	filename := filepath.Join(path, fmt.Sprintf("%s%d%s", DefaultLogName, num, suffix))
	return os.OpenFile(filename, os.O_RDWR|os.O_CREATE, os.ModePerm)
}

func createDirIfNotExist(path string) error {
	file, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.Mkdir(path, os.ModePerm)
	}

	if err != nil {
		return err
	}

	if !file.IsDir() {
		return fmt.Errorf("%s is not dir", file.Name())
	}

	return ErrorLogNeedRecover
}

func (rw *readWriterImpl) initIndexHeader() error {
	header := make([]byte, IndexHeaderSize)

	_, err := rw.index.Write(header)
	if err != nil {
		return err
	}
	return nil
}

func (rw *readWriterImpl) NextSequenceNumber() uint64 {
	return rw.baseSequenceNumber + rw.totalKeyNumber
}

func (rw *readWriterImpl) nextIndexNumber(size int) uint64 {
	return newIndexData(rw.dataFileNumber, rw.offset, uint32(size)).parseToIndexNumber()
}

func (rw *readWriterImpl) increTotalKeyNum() error {
	rw.totalKeyNumber++
	keyNum := bytes.NewBuffer(make([]byte, 0, HeaderNumSize))
	if err := binary.Write(keyNum, binary.BigEndian, rw.totalKeyNumber); err != nil {
		return err
	}

	_, err := rw.index.WriteAt(keyNum.Bytes(), HeaderNumSize)
	if err != nil {
		return err
	}

	return nil
}

func (rw *readWriterImpl) growOffset(size int) error {
	rw.offset += uint32(size)

	indexNumber := newIndexData(rw.dataFileNumber, rw.offset, 0).parseToIndexNumber()

	offset := bytes.NewBuffer(make([]byte, 0, HeaderNumSize))
	if err := binary.Write(offset, binary.BigEndian, indexNumber); err != nil {
		return err
	}

	_, err := rw.index.WriteAt(offset.Bytes(), HeaderNumSize*2)
	if err != nil {
		return err
	}

	return nil
}

func (rw *readWriterImpl) appendData(f *os.File, data []byte) (int, error) {
	n, err := f.Seek(0, os.SEEK_END)
	if err != nil {
		return 0, err
	}

	return f.WriteAt(data, n)
}

func (rw *readWriterImpl) Write(sequenceNumber uint64, data []byte) (int, error) {
	n, err := rw.appendData(rw.data, data)
	if err != nil {
		return 0, err
	}

	indexData := bytes.NewBuffer(make([]byte, 0, HeaderNumSize))
	if err := binary.Write(indexData, binary.BigEndian, rw.nextIndexNumber(n)); err != nil {
		return 0, err
	}

	_, err = rw.appendData(rw.index, indexData.Bytes())
	if err != nil {
		return 0, err
	}

	if err := rw.growOffset(n); err != nil {
		return 0, err
	}

	if err := rw.increTotalKeyNum(); err != nil {
		return 0, err
	}
	return n, nil
}

func (rw *readWriterImpl) ListBefore(sequenceNumber uint64) ([][]byte, error) {
	indexData, err := rw.findIndexDataBeforeSequenceNumber(sequenceNumber)
	if err != nil {
		return nil, err
	}

	res := make([][]byte, 0)

	indexDataMap := rw.groupIndexDataByDataFileNumber(indexData)
	for k, v := range indexDataMap {
		dataBytesArray, err := rw.readDataByIndexData(k, v)
		if err != nil {
			return nil, err
		}

		res = append(res, dataBytesArray...)
	}

	return res, nil
}

func (rw *readWriterImpl) ListAfter(sequenceNumber uint64) ([][]byte, error) {
	return nil, nil
}

func (rw *readWriterImpl) DropBefore(sequenceNumber uint64) (int, error) {
	return 0, nil
}

func (rw *readWriterImpl) Info() {
	fmt.Println("************INFO***********")

	f, _ := os.OpenFile(rw.index.Name(), os.O_RDWR|os.O_CREATE, os.ModePerm)
	header, err := io.ReadAll(f)
	fmt.Println("          ", err)
	fmt.Println(header)

	fmt.Printf("index number : %d\n", rw.indexFileNumber)
	fmt.Printf("index header :\noffset:%d\tdataNumber:%d\tbaseSequenceNumber:%d\ttotalKeyNumber:%d\n",
		rw.offset, rw.dataFileNumber, rw.baseSequenceNumber, rw.totalKeyNumber)

	fmt.Println("------------DATA-----------")
	d, _ := os.OpenFile(rw.data.Name(), os.O_RDWR|os.O_CREATE, os.ModePerm)
	data, _ := io.ReadAll(d)
	fmt.Println(len(data))
	for i := 0; i < len(data); i += 6 {
		fmt.Println(string(data[i : i+6]))
	}

	fmt.Println("------------DATA-----------")

	fmt.Println("************INFO***********")
}

func (rw *readWriterImpl) Close() {
}

func (rw *readWriterImpl) readDataByIndexData(dataFileNumber uint16, indexDataArray []*indexData) ([][]byte, error) {
	f, err := openLogFile(rw.path, DefaultDataFileName, dataFileNumber)
	if err != nil {
		return nil, err
	}
	res := make([][]byte, len(indexDataArray))

	for i, v := range indexDataArray {
		res[i] = make([]byte, v.size)
		_, err := f.ReadAt(res[i], int64(v.offset))
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (rw *readWriterImpl) groupIndexDataByDataFileNumber(data []*indexData) map[uint16][]*indexData {
	indexDataMap := make(map[uint16][]*indexData)

	for _, v := range data {
		_, ok := indexDataMap[v.dataFileNumber]
		if !ok {
			indexDataArray := make([]*indexData, 0)
			indexDataMap[v.dataFileNumber] = indexDataArray
		}

		indexDataMap[v.dataFileNumber] = append(indexDataMap[v.dataFileNumber], v)
	}

	return indexDataMap
}

func (rw *readWriterImpl) findIndexDataBeforeSequenceNumber(sequenceNumber uint64) ([]*indexData, error) {
	if sequenceNumber > rw.baseSequenceNumber+rw.totalKeyNumber {
		sequenceNumber = rw.baseSequenceNumber + rw.totalKeyNumber
	}

	indexDataLength := HeaderNumSize * (sequenceNumber - rw.baseSequenceNumber)
	indexDataArray := make([]*indexData, 0)

	indexDataArrayBytes := make([]byte, indexDataLength)
	_, err := rw.index.ReadAt(indexDataArrayBytes, IndexHeaderSize)
	if err != nil {
		return nil, err
	}

	for offset := 0; offset < int(indexDataLength); offset += HeaderNumSize {
		indexDataArray = append(indexDataArray, parseIndexData(indexDataArrayBytes[offset:offset+HeaderNumSize]))
	}
	return indexDataArray, nil

}

func (rw *readWriterImpl) findIndexDataBySequenceNumber(sequenceNumber uint64) (*indexData, error) {
	if sequenceNumber > rw.baseSequenceNumber+rw.totalKeyNumber {
		sequenceNumber = rw.baseSequenceNumber + rw.totalKeyNumber
	}

	indexDataOffset := IndexHeaderSize + HeaderNumSize*(sequenceNumber-rw.baseSequenceNumber)

	indexDataBytes := make([]byte, HeaderNumSize)
	_, err := rw.index.ReadAt(indexDataBytes, int64(indexDataOffset))
	if err != nil {
		return nil, err
	}

	return parseIndexData(indexDataBytes), nil
}

func (rw *readWriterImpl) findIndexLogBySequenceNumber(sequenceNumber uint64) int {
	// todo many index log
	return 0
}

type indexData struct {
	dataFileNumber uint16
	offset         uint32
	size           uint32
}

func newIndexData(dataFileNumber uint16, offset, size uint32) *indexData {
	return &indexData{
		dataFileNumber: dataFileNumber,
		offset:         offset,
		size:           size,
	}
}

func (d *indexData) parseToIndexNumber() uint64 {
	var indexNumber uint64 = 0

	indexNumber |= uint64(d.dataFileNumber)
	indexNumber <<= OffsetShift

	indexNumber |= uint64(d.offset)
	indexNumber <<= OffsetShift

	indexNumber |= uint64(d.size)

	return indexNumber
}

func parseIndexData(data []byte) *indexData {
	indexNum := binary.BigEndian.Uint64(data)
	shiftNum := uint64(1<<OffsetShift - 1)

	size := shiftNum & indexNum
	indexNum = indexNum >> OffsetShift

	offset := shiftNum & indexNum
	indexNum = indexNum >> OffsetShift

	dataFileNumber := indexNum

	res := &indexData{
		dataFileNumber: uint16(dataFileNumber),
		offset:         uint32(offset),
		size:           uint32(size),
	}

	return res
}
