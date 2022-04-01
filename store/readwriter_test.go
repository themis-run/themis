package store

import (
	"fmt"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestWrite(t *testing.T) {
	var data = []string{
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

func TestListBefore(t *testing.T) {
	var data = []string{
		"sadfghf",
		"",
		"123",
	}

	rw, err := NewReadWriter("./log")
	if err != nil {
		t.Fatal(err)
	}

	sequenceNumber := rw.NextSequenceNumber()
	res, err := rw.ListBefore(sequenceNumber)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4*1024; i++ {
		for i, v := range res {
			if data[i%len(data)] != string(v) {
				fmt.Printf("data: %s    v: %s\n", data[i%3], string(v))
			}
		}
	}

	rand.Seed(time.Now().Unix())
	randNum := 3 * rand.Int63n(1024)
	res, err = rw.ListBefore(uint64(randNum))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4*1024; i++ {
		for i, v := range res {
			if data[i%len(data)] != string(v) {
				fmt.Printf("data: %s    v: %s\n", data[i%len(data)], string(v))
			}
		}
	}
}

func TestListAfter(t *testing.T) {
	var data = []string{
		"sadfghf",
		"",
		"123",
	}

	rw, err := NewReadWriter("./log")
	if err != nil {
		t.Fatal(err)
	}

	res, err := rw.ListAfter(0)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4*1024; i++ {
		for i, v := range res {
			if data[i%len(data)] != string(v) {
				fmt.Printf("data: %s    v: %s\n", data[i%3], string(v))
			}
		}
	}

	rand.Seed(time.Now().Unix())
	randNum := 3 * rand.Int63n(1024)
	res, err = rw.ListBefore(uint64(randNum))
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 4*1024; i++ {
		for i, v := range res {
			if data[i%len(data)] != string(v) {
				fmt.Printf("data: %s    v: %s\n", data[i%len(data)], string(v))
			}
		}
	}

	os.RemoveAll("./log")
}
