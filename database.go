package main

import (
	"bufio"
	"github.com/dgraph-io/badger"
	"net/http"
)

func setValue(key string, value string) {
	_ = dbconfig.db.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), []byte(value))
		return err
	})
}

func getValue(key string) string {
	var valCopy []byte
	_ = dbconfig.db.View(func(txn *badger.Txn) error {
		item, _ := txn.Get([]byte(key))

		_ = item.Value(func(val []byte) error {
			valCopy = append([]byte{}, val...)
			return nil
		})
		return nil
	})
	return string(valCopy)
}

func p2pSetValue(rw *bufio.ReadWriter) {
}

func p2pGetValue(rw *bufio.ReadWriter) {
}

func httpSetValueHandler(w http.ResponseWriter, r *http.Request) {
}

func httpGetValueHandler(w http.ResponseWriter, r *http.Request) {
}
