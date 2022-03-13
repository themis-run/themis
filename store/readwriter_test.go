package store

import (
	"testing"
)

func TestWrite(t *testing.T) {
	data := []string{
		"sadfghf",
		"",
		"123",
	}

	rw, err := NewReadWriter("./log")
	if err != nil {
		t.Fatal(err)
	}

	sequenceNumber := rw.NextSequenceNumber()

	for i := 0; i < 4*1024; i++ {
		for _, v := range data {
			_, err := rw.Write(sequenceNumber, []byte(v))
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}
