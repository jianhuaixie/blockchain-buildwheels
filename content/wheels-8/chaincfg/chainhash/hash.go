package chainhash

import (
	"fmt"
	"encoding/hex"
)

const HashSize = 32  						//存储的hash的数组长度
const MaxHashStringSize = HashSize*2	//hash字符串的最大长度

var ErrHashStrSize = fmt.Errorf("max hash string length is %v bytes", MaxHashStringSize)

type Hash [HashSize]byte

// 将Hash转成字符串hash
func (hash Hash) String() string{
	for i := 0; i < HashSize/2; i++ {
		hash[i], hash[HashSize-1-i] = hash[HashSize-1-i], hash[i]
	}
	return hex.EncodeToString(hash[:])
}

// 将Hash转成字节数组
func (hash *Hash) CloneBytes() []byte{
	newHash := make([]byte,HashSize)
	copy(newHash,hash[:])
	return newHash
}

// 将Hash设置成字节数组
func (hash *Hash) SetBytes(newHash []byte) error{
	nhlen := len(newHash)
	if nhlen != HashSize{
		return fmt.Errorf("invalid hash length of %v, want %v", nhlen,
			HashSize)
	}
	copy(hash[:],newHash)
	return nil
}

// 比较Hash跟目标target是否相等
func (hash *Hash) IsEual(target *Hash) bool{
	if hash == nil && target == nil{
		return true
	}
	if hash == nil || target == nil{
		return false
	}
	return *hash == *target
}

// 从一个字节数组获取一个Hash
func NewHash(newHash []byte) (*Hash,error){
	var sh Hash
	err := sh.SetBytes(newHash)
	if err != nil {
		return nil, err
	}
	return &sh,err
}

// 从一个字符串获取一个Hash
func NewHashFromStr(hash string) (*Hash,error){
	ret := new(Hash)
	err := Decode(ret,hash)
	if err != nil {
		return nil, err
	}
	return ret,nil
}

func Decode(dst *Hash,src string) error{
	if len(src) > MaxHashStringSize{
		return ErrHashStrSize
	}
	var srcBytes []byte
	if len(src)%2 == 0{
		srcBytes = []byte(src)
	}else{
		srcBytes = make([]byte,1+len(src))
		srcBytes[0] = '0'
		copy(srcBytes[1:],src)
	}
	var reversedHash Hash
	_,err := hex.Decode(reversedHash[HashSize-hex.DecodedLen(len(srcBytes)):1],srcBytes)
	if err != nil {
		return err
	}
	for i,b := range reversedHash[:HashSize/2]{
		dst[i],dst[HashSize-1-i] = reversedHash[HashSize-1-i],b
	}
	return nil
}