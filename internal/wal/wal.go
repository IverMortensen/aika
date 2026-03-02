package wal

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"
	"sync"
)

type WAL struct {
	file *os.File
	mu   sync.Mutex
	buf  *bufio.Writer
}

// Create new or open existing WAL
func Open(path string) (*WAL, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &WAL{
		file: f,
		buf:  bufio.NewWriter(f),
	}, nil
}

// Close WAL
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Write any buffered data
	if err := w.buf.Flush(); err != nil {
		return err
	}

	return w.file.Close()
}

// Appends an entry to the WAL
func (w *WAL) Write(data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	checksum := crc32.ChecksumIEEE(data)
	length := uint32(len(data))

	var header [8]byte
	binary.LittleEndian.PutUint32(header[0:4], length)
	binary.LittleEndian.PutUint32(header[4:8], checksum)

	// Write entry to buffer: [length][checksum][data]
	if _, err := w.buf.Write(header[:]); err != nil {
		return err
	}
	if _, err := w.buf.Write(data); err != nil {
		return err
	}

	// Write the buffer to the file
	if err := w.buf.Flush(); err != nil {
		return err
	}

	// Make sure the file is written to disk
	// NOTE: This is slow. Consider batching
	return w.file.Sync()
}

func (w *WAL) Replay(fn func(data []byte) error) error {
	// Read the file to memory
	// NOTE: The file should not be too big.
	// If it is, implement snapshots or pruning to reduce size
	file, err := os.ReadFile(w.file.Name())
	if err != nil {
		return err
	}
	fileLen := uint64(len(file))

	// Loop through each entry of the file
	var offset uint64
	var lastGoodOffset uint64
	for offset < fileLen {
		if offset+8 > fileLen {
			return w.truncateTo(lastGoodOffset)
		}

		length := binary.LittleEndian.Uint32(file[offset:])
		checksum := binary.LittleEndian.Uint32(file[offset+4:])

		if offset+8+uint64(length) > fileLen {
			return w.truncateTo(lastGoodOffset)
		}

		data := file[offset+8 : offset+8+uint64(length)]

		if crc32.ChecksumIEEE(data) != checksum {
			// TODO: Figure out if truncate is fine or if we should error
			return w.truncateTo(lastGoodOffset)
			// return fmt.Errorf("checksum mismatch at offset %d", offset)
		}

		// Run the provided function on the current entry
		if err := fn(data); err != nil {
			return err
		}

		offset += 8 + uint64(length)
		lastGoodOffset = offset
	}

	return nil
}

// Truncate the file to an offset
func (w *WAL) truncateTo(offset uint64) error {
	if err := w.file.Truncate(int64(offset)); err != nil {
		return fmt.Errorf("failed to truncate WAL to offset %d: %w", offset, err)
	}
	w.buf = bufio.NewWriter(w.file) // reset buffer
	return w.file.Sync()
}
