//go:build cgo

package memory

// #cgo CFLAGS: -DSQLITE_CORE
// #cgo linux LDFLAGS: -lm
// #include "sqlite-vec.h"
import "C"
import (
	"bytes"
	"encoding/binary"
)

// sqliteVecAuto registers sqlite-vec as an auto-extension for all future
// SQLite connections in this process. Must be called before opening any DB.
func sqliteVecAuto() {
	C.sqlite3_auto_extension((*[0]byte)((C.sqlite3_vec_init)))
}

// serializeFloat32 encodes a float32 slice into the little-endian blob
// format that sqlite-vec expects for vector columns.
func serializeFloat32(vector []float32) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, vector)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
