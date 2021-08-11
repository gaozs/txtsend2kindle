package main

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestBase64Wrap(t *testing.T) {
	buf := []byte("this is a just a test")
	out := new(bytes.Buffer)
	err := base64Wrap(out, bytes.NewBuffer(buf))
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != base64.StdEncoding.EncodedLen(len(buf))+len("\r\n") {
		t.Fatal("len mismatch")
	}
	t.Log(out.String(), out.Len())

	buf = make([]byte, 57)
	out = new(bytes.Buffer)
	err = base64Wrap(out, bytes.NewBuffer(buf))
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != base64.StdEncoding.EncodedLen(len(buf))+len("\r\n") {
		t.Fatal("len mismatch")
	}
	t.Log(out.String(), out.Len())

	buf = make([]byte, 0)
	out = new(bytes.Buffer)
	err = base64Wrap(out, bytes.NewBuffer(buf))
	if err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatal("len mismatch")
	}
	t.Log(out.String(), out.Len())
}
