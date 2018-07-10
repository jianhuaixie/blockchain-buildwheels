### 交易标准

为了让所有节点都能识别交易，因此将交易信息标准化是必不可少的，比特币的区块链社区支持五种交易标准：
P2PKH,P2PK,MS,P2SH和OP_Return。OP_Return因为其存储的是信息，并非比特币货币，就不讲了。

### 比特币脚本语言

所有节点都能理解的一种语言，并且按照步骤进行编译。发送方只要将自己的UTXO使用锁定脚本进行锁定，锁定在接收方的地址上，
表示只有证明自己是接收方，就能使用地址上的余额。接收方如果要使用，必须使用解锁脚本，证明自己是是地址的主人，各个节点才
认可这笔交易。

在比特币社区使用的脚本语言，是一种基于逆波兰表示法的基于堆栈的执行语言。

脚本语言是一种类Forth栈语言，被设计拥有成无状态和非图灵完备的性质。无状态性保证了一旦一个交易被区块打包，这个交易就是可用的。
图灵非完备性（缺少循环和goto语句）使得比特币的脚本语言更加不灵活和更可预测，从而大大简化了安全模型。

比如执行 2+3==5 这样一个表达式：

![](http://s3.sinaimg.cn/middle/002RSgYjzy7lxhyTnG292&690)

### P2PKH(Pay-to-Public-Key-Hash)  支付到公钥哈希

比特币网络上，大部分交易是以P2PKH的方式进行的，下面是P2PKH的锁定脚本与解锁脚本：

![](http://s5.sinaimg.cn/middle/002RSgYjzy7lxhXpMTqb4&690)

<sig>表示签名， <PubK>表示Public Key,具体按照逆波兰表达式的操作步骤如下：

![](http://s3.sinaimg.cn/middle/002RSgYjzy7lxhZmJIC42&690)

从上面的操作步骤可以看出，主要是验证两个东西，第一个是Public Key是否能够转换成正确的地址，
第二个是Signature是否正确，也就是证明你是否是这个Public Key的主人。

###### Signature签名

签名内容主要是此交易摘要（交易信息的Hash）与私钥进行运算。验证的话，将签名与公钥进行运算，如果能正确得到交易hash，则成功。

### P2PK(Pay-to-Public-Key)

P2PK锁定脚本形式如下：

> <Public Key A> OP_CHECKSIG

用于解锁的脚本是一个简单签名：

> <Signature from Private Key A>

经由交易验证软件确认的组合脚本为：

> <Signature from Private Key A> <Public Key A> OP_CHECKSIG

根据上面的规则运行可以简单发现，P2PK比P2PKH简单得多，只有一步验证，少了前面P2PKH的地址验证。

### MS(Multiple Signatures) 多重签名脚本

通用的M-N多重签名锁定脚本形式为：

> M <Public Key 1> <Public Key 2> ... <Public Key N> M OP_CHECKMULTISIG

其中，N是存档公钥总数，M是要求激活交易的最少公钥数。

简单例子，2-3多重签名条件：

> <Public Key A> <Public Key B> <Public Key C> 3 OP_CHECKMULTISIG

上面的锁定脚本可由含有签名和公钥的脚本予以锁定：

> OP_0 <Signature B> <Signature C>

OP_0 是站位符，没有实际意义。两个脚本组合将形成一个验证脚本：

> OP_0 <Signature B> <Signature C> 2 <Public Key A> <Public Key B> <Public Key C> 3 OP_CHECKMULTISIG

### P2SH (Pay-to-Script-Hash) 支付到脚本公钥

P2SH用来向一个脚本hash发起交易，每一个标准公钥脚本可以视作一个P2SH兑换脚本（赎回脚本）。

P2SH是MS多重签名的简化版本，如果使用P2SH进行和上面相同的多重签名条件，步骤如下：

> 2 <Public Key A> <Public Key B> <Public Key C> 3 OP_CHECKMULTISIG

对锁定脚本，首先采用SHA256哈希算法，随后对其运用RIPEMD160算法。20字节的脚本为：

> 8ac1d7a2fa204a16dc984fa81cfdf86a2a4e1731

锁定脚本如下：

> OP_HASH160 8ac1d7a2fa204a16dc984fa81cfdf86a2a4e1731 OP_EQUAL

当接收方需要使用此交易中的UTXO时，需要提交解锁脚本，也叫赎回脚本：

> <Sig1> <Sig2> <2 PK1 PK2 PK3 PK4 PK5 5 OP_CHECKMULTISIG>

与锁定脚本集合后：

> <Sig1> <Sig2> <2 PK1 PK2 PK3 PK4 PK5 5 OP_CHECKMULTISIG> OP_HASH160 8ac1d7a2fa204a16dc984fa81cfdf86a2a4e1731 OP_EQUAL

根据逆波兰表达式运算，可以很明显看到，验证分为两步，第一步是验证接收方附上的赎回脚本是否符合发送方的锁定脚本。
如果符合，就执行该脚本，进行多重签名的验证。

P2SH的特点是，将制作脚本的责任给了接收方，好处是可以暂缓节点存储的压力。










