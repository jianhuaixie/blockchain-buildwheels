package main

import "crypto/sha256"

// Merkle树的组成部分是一个或多个Merkle节点，而一颗Merkle树，能用根节点跟其他树区分开来
type MerkleTree struct {
	RootNode *MerkleNode
}

// Merkle节点  其结构定义就是一个递归的样子，一个根Merkle节点就能代表整个Merkle树
type MerkleNode struct {
	Left	*MerkleNode
	Right	*MerkleNode
	Data	[]byte
}

// 构建一个Merkle树，首先传入参数就是所有交易组成的数组
func NewMerkleTree(data [][]byte) *MerkleTree{
	// 第一步，声明一个用来装Merkle节点的数组
	var nodes []MerkleNode
	// 第二步，检查参数数组是否是偶数，如果是奇数，将最后一笔交易double一下
	if len(data)%2 != 0{
		data = append(data,data[len(data)-1])
	}
	// 第三步，遍历交易数组，不断构建merkle节点，将节点插入第一步声明的数组中
	for _,datum := range data{
		node := NewMerkleNode(nil,nil,datum)
		nodes = append(nodes,*node)
	}
	// 第四步，进行两次遍历，按照规则构成所需的merkle树
	for i:=0;i<len(data)/2;i++{
		var newLevel []MerkleNode
		for j:=0;j<len(nodes);j+=2{
			node := NewMerkleNode(&nodes[j],&nodes[j+1],nil)
			newLevel = append(newLevel,*node)
		}
		nodes = newLevel
	}
	// 第五步，将merkle树最上面的节点作为参数，构建一个merkle树
	merkleTree := MerkleTree{&nodes[0]}
	return &merkleTree
}

// 构建一个Merkle节点，传入参数需三个:左节点，右节点，数据
func NewMerkleNode(left,right *MerkleNode,data []byte) *MerkleNode{
	// 第一步，声明一个merkle节点
	merkleNode := MerkleNode{}
	// 第二步，查看左右节点是否全为空，全为空，直接将data转成256的hash值转入merkle节点的Data属性中，如果不为空，需要将左右节点的Data属性也考虑进去
	if left == nil && right == nil{
		hash := sha256.Sum256(data)
		merkleNode.Data = hash[:]
	}else{
		prevHashes := append(left.Data,right.Data...)
		hash := sha256.Sum256(prevHashes)
		merkleNode.Data = hash[:]
	}
	// 第三步，左右节点接上
	merkleNode.Left = left
	merkleNode.Right = right
	return &merkleNode
}
