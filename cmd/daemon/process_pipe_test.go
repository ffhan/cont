package main

import (
	"bytes"
	"io/ioutil"
	"testing"
)

func TestDynamicReader_Read(t *testing.T) {
	d := NewDynamicReader()

	expected0 := "test123"
	buf0 := bytes.NewBufferString(expected0)
	expected1 := "hello "
	buf1 := bytes.NewBufferString(expected1)
	expected2 := "world!"
	buf2 := bytes.NewBufferString(expected2)

	d.AddReader(ioutil.NopCloser(buf0))

	buffer := make([]byte, 64)

	n, err := d.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	result := string(buffer[:n])
	if result != expected0 {
		t.Errorf("expected %s, got %s", expected0, result)
	}

	d.AddReader(ioutil.NopCloser(buf1))
	d.AddReader(ioutil.NopCloser(buf2))

	n, err = d.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	result = string(buffer[:n])
	if result != expected1 {
		t.Errorf("expected %s, got %s", expected1, result)
	}

	n, err = d.Read(buffer)
	if err != nil {
		t.Fatal(err)
	}
	result = string(buffer[:n])
	if result != expected2 {
		t.Errorf("expected %s, got %s", expected2, result)
	}
}
