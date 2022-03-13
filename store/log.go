package store

import (
	"errors"
	"time"
)

var ErrorAppendLog = errors.New("wal append error")

const (
	DefaultLogPersistTime   = 200 * time.Second
	DefaultLogWriteFileTime = 20 * time.Millisecond
)

type Log interface {
	Append(*Event) error
	Flush()
}

type log struct {
	encoder        *encoder
	readWriter     ReadWriter
	sequenceNumber uint64
	logCache       chan []byte

	persisitTime   time.Duration
	persisitTimer  time.Timer
	writeFileTime  time.Duration
	writeFileTimer time.Timer
}

func (l *log) Append(e *Event) error {
	if e.Name != Set && e.Name != Delete {
		return nil
	}

	data, err := l.encoder.encode(e)
	if err != nil {
		return err
	}

	l.logCache <- data
	return nil
}

func (l *log) Flush() {

}

func (l *log) startLog() {
	for {
		select {
		case <-l.persisitTimer.C:
			l.persisitTimer.Reset(l.persisitTime)
			l.Flush()

		case <-l.writeFileTimer.C:
			l.writeFileTimer.Reset(l.writeFileTime)

			for i := 0; i < len(l.logCache); i++ {
				data := <-l.logCache
				n, err := l.readWriter.Write(l.sequenceNumber, data)
				if err != nil || len(data) != n {
					break
				}
				l.sequenceNumber++
			}
		}
	}
}
