package main

import "crypto/sha256"

type MerkleTree struct{
	RootNode *MerkleNode
}
type MerkleNode struct{
	Left *MerkleNode
	Right *MerkleNode
	Data []byte
}

// creates a new Merkle tree from a sequence of data
func NewMerkleTree(data [][]byte) *MerkleTree{
	var nodes []MerkleNode
	if len(data)%2 != 0{
		data = append(data,data[len(data)-1])
	}
	for _,datum := range data{
		node := NewMerkleNode(nil,nil,datum)
		nodes = append(nodes,*node)
	}
	for i:=0;i<len(data)/2;i++{
		var newLevel []MerkleNode
		for j:=0;j<len(nodes);j+=2{
			node := NewMerkleNode(&nodes[j],&nodes[j+1],nil)
			newLevel = append(newLevel,*node)
		}
		nodes = newLevel
	}
	merkleTree := MerkleTree{&nodes[0]}
	return &merkleTree
}

// creates a new Merkle tree node
func NewMerkleNode(left,right *MerkleNode,data []byte) *MerkleNode{
	merkleNode := MerkleNode{}
	if left == nil && right == nil{
		hash := sha256.Sum256(data)
		merkleNode.Data = hash[:]
	}else{
		prevHashes := append(left.Data,right.Data...)
		hash := sha256.Sum256(prevHashes)
		merkleNode.Data = hash[:]
	}
	merkleNode.Left = left
	merkleNode.Right = right
	return &merkleNode
}