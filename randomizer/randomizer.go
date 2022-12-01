package randomizer

import (
	"io"
	"math/rand"
	"os"
)

// RandomID represents a unique random element
type RandomID int32

// Randomizer to return random pages of blocksize
type Randomizer struct {
	buffer []byte
}

const (
	// Blockshift is the bit width for block size
	Blockshift = 12
	// Blocksize is the size for the returned elements
	Blocksize = 1 << Blockshift

	poolsize = 16 * 1024 * 1024
)

// NewRandomizer creates a new randomizer
func NewRandomizer() (*Randomizer, error) {
	r := &Randomizer{
		buffer: make([]byte, poolsize),
	}

	file, err := os.Open("/dev/urandom")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = io.ReadFull(file, r.buffer)

	return r, err
}

// GetRandom returns a new RandomID with blocksize bytes
func (r *Randomizer) GetRandom() (RandomID, []byte) {
	id := rand.Int31n(poolsize - Blocksize)

	return RandomID(id), r.buffer[id : id+Blocksize]
}

// GetByID returns the random data associated with id
func (r *Randomizer) GetByID(id RandomID) []byte {
	return r.buffer[id : id+Blocksize]
}
