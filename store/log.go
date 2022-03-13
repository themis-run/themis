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
	Recover() []*Event
	Flush()
}

type log struct {
	*decoder
	*encoder
	readWriter     ReadWriter
	sequenceNumber uint64
	logCache       chan []byte
	persistCh      chan struct{}

	persisitTime   time.Duration
	persisitTimer  time.Timer
	writeFileTime  time.Duration
	writeFileTimer time.Timer
}

func (l *log) Append(e *Event) error {
	if e.Name != Set && e.Name != Delete {
		return nil
	}

	data, err := l.encode(e)
	if err != nil {
		return err
	}

	l.logCache <- data
	return nil
}

func (l *log) Recover() ([]*Event, error) {
	data, err := l.readWriter.ListAfter(0)
	if err != nil {
		return nil, err
	}

	events := make([]*Event, 0)
	for _, v := range data {
		event := l.decode(v)
		events = append(events, event)
	}

	return events, nil
}

func (l *log) Flush() {
	l.persistCh <- struct{}{}
}

func (l *log) startLog() {
	for {
		select {
		case <-l.persisitTimer.C:
			l.persisitTimer.Reset(l.persisitTime)
			l.Flush()

		case <-l.writeFileTimer.C:
			l.writeFileTimer.Reset(l.writeFileTime)
			l.doPersist()
		case <-l.persistCh:
			l.doPersist()
		}
	}
}

func (l *log) doPersist() {
	for i := 0; i < len(l.logCache); i++ {
		data := <-l.logCache
		n, err := l.readWriter.Write(l.sequenceNumber, data)
		if err != nil || len(data) != n {
			break
		}
		l.sequenceNumber++
	}
}
