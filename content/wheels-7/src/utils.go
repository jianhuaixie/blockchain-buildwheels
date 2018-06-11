package main

import (
	"bytes"
	"encoding/binary"
	"log"
)

// 将int64转成字节数组
func IntToHex(num int64) []byte{
	buff := new(bytes.Buffer)
	err := binary.Write(buff,binary.BigEndian,num)
	if err != nil {
		log.Panic(err)
	}
	return buff.Bytes()
}

// 将字节数组反转
func ReverseBytes(data []byte){
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}
