package main

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"testing"
)

type closerBufferString struct {
	bytes.Buffer
}

func NewCloserBufferString(s string) *closerBufferString {
	c := &closerBufferString{}
	c.WriteString(s)
	return c
}

func (c *closerBufferString) Close() error {
	return nil
}

func TestDynamicPipe_Read_BigBuffer(t *testing.T) {
	pipe := NewDynamicPipe()

	pipe.Add(NewCloserBufferString("hello world!"))
	pipe.Add(NewCloserBufferString("test123"))

	b := make([]byte, 2048)
	n, err := pipe.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b[:n]))
	n, err = pipe.Read(b)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(string(b[:n]))
	n, err = pipe.Read(b)
	if !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

func TestDynamicPipe_Read_SmallBuffer(t *testing.T) {
	pipe := NewDynamicPipe()

	pipe.Add(NewCloserBufferString("0123456789"))
	pipe.Add(NewCloserBufferString("abcdef"))

	b := make([]byte, 4)
	for i := 0; i < 5; i++ {
		n, err := pipe.Read(b)
		if err != nil {
			t.Fatal(err)
		}
		t.Log(string(b[:n]))
	}
	_, err := pipe.Read(b)
	if !errors.Is(err, io.EOF) {
		t.Fatal(err)
	}
}

func TestDynamicPipe_Write(t *testing.T) {
	pipe := NewDynamicPipe()

	b1 := &closerBufferString{}
	b2 := &closerBufferString{}

	pipe.Add(b1)
	pipe.Add(b2)

	expected := "test123"
	_, err := pipe.Write([]byte(expected))
	if err != nil {
		t.Fatal(err)
	}

	result, err := ioutil.ReadAll(b1)
	if err != nil {
		t.Error(err)
	}
	resultString := string(result)
	if resultString != expected {
		t.Errorf("expected %s, got %s", expected, resultString)
	}

	result, err = ioutil.ReadAll(b2)
	if err != nil {
		t.Error(err)
	}
	resultString = string(result)
	if resultString != expected {
		t.Errorf("expected %s, got %s", expected, resultString)
	}
}
