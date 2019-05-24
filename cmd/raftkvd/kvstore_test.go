package main

import (
	"reflect"
	"testing"
)

func Test_kvstore_snapshot(t *testing.T) {
	tm := map[string]string{"foo": "bar"}
	s := &kvstore{kvStore: tm}

	v, _ := s.Lookup("foo")
	if v != "bar" {
		t.Fatalf("foo has unexpected value, got %s", v)
	}

	data, err := s.getSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	s.kvStore = nil

	if err := s.recoverFromSnapshot(data); err != nil {
		t.Fatal(err)
	}
	v, _ = s.Lookup("foo")
	if v != "bar" {
		t.Fatalf("foo has unexpected value, got %s", v)
	}
	if !reflect.DeepEqual(s.kvStore, tm) {
		t.Fatalf("store expected %+v, got %+v", tm, s.kvStore)
	}
}
