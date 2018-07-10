package chainhash

import "crypto/sha256"

func HashB(b []byte) []byte{
	hash := sha256.Sum256(b)
	return hash[:]
}

func HashH(b []byte) Hash{
	return Hash(sha256.Sum256(b))
}

func DoubleHashB(b []byte) []byte{
	first := sha256.Sum256(b)
	second := sha256.Sum256(first[:])
	return second[:]
}

func DoubleHashH(b []byte) Hash{
	first := sha256.Sum256(b)
	return Hash(sha256.Sum256(first[:]))
}

