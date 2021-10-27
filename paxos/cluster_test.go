package main

import (
	"encoding/gob"
	"net"
	"testing"
)

func TestNodeConnection(t *testing.T) {
	message := "Hello!"
	expected := "Start"

	for i := 0; i < 3; i++ {
		master, _ := net.ResolveTCPAddr("tcp4", "localhost:1234")
		conn, err := net.DialTCP("tcp4", nil, master)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		err = gob.NewEncoder(conn).Encode(&message)
		if err != nil {
			t.Fatal(err)
		}
		if i == 2 {
			err = gob.NewDecoder(conn).Decode(&message)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	got := message // expect Start
	if message != got {
		t.Errorf("TestNodeConnection, expected %s, but got %s ", expected, got)
	}
}
