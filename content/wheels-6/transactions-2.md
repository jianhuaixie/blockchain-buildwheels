# 交易（2）

在之前版本的轮子中，都没有体现数据库为分布式，这一个版本的轮子将重点放在区块链的分布式特性上。

### 奖励

挖矿奖励是必须的，要不整个区块链就不存在激励。挖矿奖励，实际上就是一笔coinbase交易。当一个挖矿节点开始挖出一个新块时，将
交易从队列中取出，并在前面附加一笔coinbase交易。coinbase交易只有一个输出，里面包含了矿工的公钥哈希。

### UTXO集

区块被存储在blocks数据库，交易输出被存储在chainstate数据库。chainstate的结构：

- 1.c+32字节的交易哈希 -> 该笔交易的未花费交易输出记录
- 2.B+32字节的块哈希 -> 未花费交易输出的块哈希

chainstate不存储交易，存储的是UTXO集，也就是未花费交易输出的集合。除此之外，还存储了“数据库表示的未花费交易输出的块哈希”，
至于块哈希这个概念，会在下一个版本的轮子中用到块高度。

Blockchain.FindUnspentTransactions这个方法是迭代整个区块链里面的每一个区块，检查每一笔交易，就为了找到那些未花费的输出。

其实我们仅仅需要一个未花费输出的索引就可以了，这就是UTXO集要做的事情：这是一个从所有区块链交易中构建（对区块进行迭代，但只要做一次）
而来的缓存，然后用来计算余额和验证新的交易。

UTXO集没法存储所有交易，只会存储那些未花费输出的交易。因此，无法用于Blockchain.FindTransaction。

这个版本的轮子需要一下方法：

- 1.Blockchain.FindUTXO   通过对区块进行迭代找到所有未花费输出
- 2.UTXOSet.Reindex 使用UTXO找到未花费输出，然后再唉数据库中进行存储，这里就是缓存的地方
- 3.UTXOSet.FindSpendableOutputs    类似Blockchain.FindSpendableOutputs，使用UTXO集来做
- 4.UTXOSet.FindUTXO    类似Blockchain.FindUTXO，但是使用UTXO集来做
- 5.Blockchain.FindTransaction  跟之前一样

    type UTXOSet struct{
        BLockchain  *Blockchain
    }

有了UTXO集，意味着我们的交易数据会被分开存储：实际交易被存储在区块链中，未花费输出被存储在UTXO集中。

需要一个良好的同步机制，UTXO集时刻出于最新状态，并且存储最新交易的输出。也并不是一生成新块时，就要重新索引，这是会对区块链
进行扫描的重操作，需要一个轻巧的机制来更新UTXO集。

### Merkle树

在这个版本中，需要实现这个Merkle树的优化机制。

Merkle树的引用背景是什么呢？

比特币区块链全节点越来越大，想要实现不需要下载整个区块链，也不需要验证区块和交易。只需要在区块链检查交易，并且需要连接
到一个全节点来检索必要的数据，这个机制允许在仅运行一个全节点的情况下有多个轻钱包。

###### SPV是什么？

简易支付验证（Simplified Payment Verification,SPV）,为了实现SPV，需要有一个方式来检查是否有一个区块包含了某笔交易，而无需
下载整个区块。

比特币用Merkle树来获取交易哈希，哈希被保存在区块头中，并不会用于工作量证明系统。在之前版本的轮子中，区块链仅仅把每笔交易
哈希连接了起来，没有利用到Merkle树。

观察一下Merkle树：

![](http://chuantu.biz/t6/324/1528278618x1822611335.jpg)

每个块都会有一个Merkle树，从叶子节点开始，一个叶子节点就是一个交易哈希。叶子节点的数量必须是双数。

从下往上，两两成对，连接两个节点哈希，将组合哈希作为新的哈希。新的哈希就成为i新的树节点。重复该过程，直到仅有一个节点，
也就是树根。根哈希就可以的那工作整个块交易的唯一标识，将其保存到区块头，用于工作量证明。

Merkle树的好处是一个节点可以在不下载整个块的情况下，验证是否包含某笔交易。并且这些只需要一个交易哈希，一个Merkle树根哈希和
一个Merkle路径。

看看MerkleTree和MerkleNode

    type MerkeTree struct{
        RootNode *MerkleNode
    }
    type MerkleNode struct{
        Left    *MerkleNode
        Right   *MerkleNode
        Data    []byte
    }

从上面可以看出，MerkleNode包含数据和指向左右分支的指针。MerkleTree实际上就是连接到下个节点的根节点，然后依次连接到更远的节点。

### P2PKH

在比特币中有一个脚本（script）编程语言，用于锁定交易输出，交易输入提供了解锁输出的数据。这个语言非常简单，用这个语言的代码其实就是一系列
数据和操作符而已。比如下面：

    5 2 OP_ADD 7 OP_EQUAL

脚本代码从左到右执行：将数据依次放入栈内，当遇到操作符时，就从栈内取出数据，并将操作符作用于数据，然后将结果作为栈顶元素。
脚本的栈，实际上就是一个先进后出的内存存储。

分析一下比特币如何用脚本执行支付的：

    <signature> <pubKey> OP_DUP OP_HASH160 <pubKeyHash> OP_EQUALVERIFY OP_CHECKSIG

这个脚本叫Pay to Public Key Hash（P2PKH），完成的事情就是向一个公钥哈希支付，用一个公钥所定义一些币，这是比特币支付的
核心：没有账户，没有资金转移，只有一个脚本检查提供的签名和公钥是否正确。

这个脚本实际存储两个部分：

- 1.第一个部分，<signature> <pubkey> ，存储在输入的ScriptSig字段
- 2.第二部分，OP_DUP OP_HASH160 <pubkeyHash> OP_EQUALVERYFY OP_CHECKSIG 存储在输出的ScriptPubKey里面

OP_DUP 对栈顶元素进行复制。OP_HASH160取栈顶元素，然后用RIPEMD160进行哈希
OP_EQUALVERIFY 将栈顶的两个元素进行比较，如果不相等，终止脚本。
OP_CHECKSIG通过对交易进行哈希，并使用<signature>和pubkey来验证一笔交易的签名。

> 有了这样的脚本语言，实际上比特币成为了一个智能合约平台，除了将一个单一的公钥转移资金，这个语言还使得一些其他的支付方案成为可能。

### 项目测试脚本

    go build -o blockchain_go
    ./blockchain_go createwallet
    ./blockchain_go createblockchain -address 13VpYJGo9T4B8JcrUDEDXe4kFwsXiaksVF
    ./blockchain_go getbalance -address 13VpYJGo9T4B8JcrUDEDXe4kFwsXiaksVF
    ./blockchain_go listaddresses
    ./blockchain_go send -from 13VpYJGo9T4B8JcrUDEDXe4kFwsXiaksVF -to 1viKv8ySy2ftzSgPPZdCKyrKTi7sHorNS -amount 2
    ./blockchain_go printchain

### 项目代码

    






















