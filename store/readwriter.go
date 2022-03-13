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
	DataNumberShift      = 16
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
	indexNumber uint32
	*indexLogHeader
	index *os.File
	data  *os.File
}

type indexLogHeader struct {
	offset             uint32
	dataNumber         uint32
	baseSequenceNumber uint64
	totalKeyNumber     uint64
}

func createReadWriter(path string) (ReadWriter, error) {
	var indexNumber uint32 = 0
	var dataNumber uint32 = 0

	index, err := openLogFile(path, DefaultIndexFileName, indexNumber)
	if err != nil {
		return nil, err
	}

	data, err := openLogFile(path, DefaultDataFileName, dataNumber)
	if err != nil {
		return nil, err
	}

	rw := &readWriterImpl{
		indexNumber: indexNumber,
		index:       index,
		data:        data,
		indexLogHeader: &indexLogHeader{
			dataNumber:         dataNumber,
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
	indexNumber := findIndexLog(path)

	index, err := openLogFile(path, DefaultIndexFileName, indexNumber)
	if err != nil {
		return nil, err
	}

	indexHeader, err := readLogHeader(index)
	if err != nil {
		return nil, err
	}

	data, err := openLogFile(path, DefaultDataFileName, indexHeader.dataNumber)
	if err != nil {
		return nil, err
	}

	rw := &readWriterImpl{
		indexNumber:    indexNumber,
		indexLogHeader: indexHeader,
		index:          index,
		data:           data,
	}
	return rw, nil
}

func readLogHeader(f *os.File) (*indexLogHeader, error) {
	header := make([]byte, IndexHeaderSize)
	_, err := f.Read(header)
	if err != nil {
		return nil, err
	}

	//header := x[:IndexHeaderSize]
	fmt.Println(header)

	baseSequenceNumber := binary.BigEndian.Uint64(header[:8])
	totalKeyNumber := binary.BigEndian.Uint64(header[8:16])
	offsetNum := binary.BigEndian.Uint64(header[16:])
	offsetNum = offsetNum >> OffsetShift
	dataNumber := offsetNum >> OffsetShift
	offset := offsetNum & (1<<(OffsetShift+1) - 1)

	return &indexLogHeader{
		offset:             uint32(offset),
		dataNumber:         uint32(dataNumber),
		baseSequenceNumber: baseSequenceNumber,
		totalKeyNumber:     totalKeyNumber,
	}, nil
}

func findIndexLog(path string) uint32 {
	var indexNumber uint32 = 0
	for {
		filename := filepath.Join(path, fmt.Sprintf("%s%d%s", DefaultLogName, indexNumber, DefaultIndexFileName))
		f, err := os.Stat(filename)
		if errors.Is(err, os.ErrNotExist) {
			return indexNumber
		}

		if f.Size() < MaxIndexLogSize {
			return indexNumber
		}

		indexNumber++
	}
}

func openLogFile(path, suffix string, num uint32) (*os.File, error) {
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

func (rw *readWriterImpl) nextIndexNum(size int) uint64 {
	var indexNum uint64

	indexNum = indexNum | uint64(rw.dataNumber)
	indexNum = indexNum << DataNumberShift

	indexNum = indexNum | uint64(rw.offset)
	indexNum = indexNum << OffsetShift

	indexNum = indexNum | uint64(size)
	return indexNum
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
	offsetNumber := uint64(rw.dataNumber)
	offsetNumber = offsetNumber << DataNumberShift
	offsetNumber |= uint64(rw.offset)
	offsetNumber = offsetNumber << OffsetShift

	offset := bytes.NewBuffer(make([]byte, 0, HeaderNumSize))
	if err := binary.Write(offset, binary.BigEndian, offsetNumber); err != nil {
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
	fmt.Println("sequenceNumber: ", sequenceNumber)
	n, err := rw.appendData(rw.data, data)
	if err != nil {
		return 0, err
	}

	indexData := bytes.NewBuffer(make([]byte, 0, HeaderNumSize))
	if err := binary.Write(indexData, binary.BigEndian, rw.nextIndexNum(n)); err != nil {
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
	return nil, nil
}

func (rw *readWriterImpl) findIndexLogBySequenceNumber(sequenceNumber uint64) int {
	// todo many index log
	return 0
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

	fmt.Printf("index number : %d\n", rw.indexNumber)
	fmt.Printf("index header :\noffset:%d\tdataNumber:%d\tbaseSequenceNumber:%d\ttotalKeyNumber:%d\n",
		rw.offset, rw.dataNumber, rw.baseSequenceNumber, rw.totalKeyNumber)

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
