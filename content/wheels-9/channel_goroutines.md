# golang的goroutine和channel的理解

### Go语言的goroutines,信道和死锁

##### goroutine

goroutine有点类似线程，但是更轻。

使用关键字go来定义并启动一个goroutine：

    func main(){
        go loop()
        loop()
        time.Sleep(time.Second) // main函数太快，为了等待一下goroutine，停顿一秒
    }

如果goroutine在结束的时候，能跟主线程能通信，那就需要信道。

##### 信道

简单来说，channel信道是goroutine之间相互通信的东西。用来goroutine之间发消息和接收消息。其实，就是在做goroutine之间的内存共享。

是用make来建立一个信道：

    var channel chan int = make(chan int)
    // 或者直接声明
    channel := make(chan int)

向信道存消息和取消息，一个例子说明：

    func main(){
        var message chan string = make(chan string)
        go func(message string){
            message <- message // 存消息
        }("Ping!")
        fmt.Println(<-message) // 取消息
    }

信道的存消息和取消息都是阻塞的，也就是说，无缓冲的信道在取消息和存消息的时候都会挂起当前的goroutine，除非另一端已经准备好。

有了这个阻塞的机制，来实现让goroutine告诉主线程：我执行完了。下面的demo很简单就做到了。

    var complete chan int = make(chan int)
    func loop(){
        for i:=0;i<10;i++{
            fmt.Printf("%d",i)
        }
        completee <- 0 // 执行完毕，发个消息
    }
    func main(){
        go loop()
        <- complete // loop()线程跑完，收到消息，main接着跑
    }

其实，无缓冲的信道永远不会存储数据，只会负责数据的流通。

- 从无缓冲信道取数据，必须要有数据流进来才可以，否则当前线程阻塞
- 数据流入无缓冲信道，如果没有其他goroutine取走数据，那么当前线程阻塞

如果信道正有数据，那还要加入数据，或者信道没有数据，硬要取数据，就会引起死锁。

##### 死锁

一个死锁的简单例子：

    func main(){
        ch := make(chan int)
        <- ch // 没有往ch信道存放数据，就开始取数据，就直接阻塞main goroutine，信道ch被锁
    }

其实简单理解就是，一个goroutine，向里面加数据或者存数据，都会锁死信道，并且阻塞当前goroutine，也就是死锁。

死锁的解决办法也很简单，没取走的数据赶紧取走，没放入的数据一定要放入，无缓冲信道不能承载数据。

另外一个解决办法就是缓冲信道，即设置channel有一个数据的缓冲大小：

    c := make(chan int,1)

这个也很好理解，c可以缓存一个数据，放入一个，不会阻塞，再放入一个，阻塞，被其他goroutine取走一个，又不阻塞。
也就是说，只阻塞在容量一定的时候，不达容量就不阻塞。

非常类似Pyhton里面的Queue的概念。

##### 无缓冲信道的数据进出顺序

一句话说清楚：无缓冲信道的数据是先到先出。

##### 缓冲信道

缓冲信道在流入的数据数目超过容量的时候，就会死锁。

可以简单把缓冲信道看成是一个线程安全的队列。

##### 信道数据读取和信道关闭

取信道消息可以用range进行遍历：

    for v := range ch{ // ch := make(chan int,3)
        fmt.Println(v)
        if len(ch) <= 0{
            break  // 为了避免信道没消息了，还去取造成死锁。
        }
    }

##### 等待多goroutine的方案

使用信道阻塞主线，其他goroutine跑完回到主线。

其实无缓冲信道和缓冲信道都能实现这个需求，伪代码如下：

    package main
    import "fmt"

    var quit chan int
    func foo(id int){
    	fmt.Println(id)
    	quit <- 0
    }
    func main(){
    	count := 1000
    	quit = make(chan int)
    	for i:=0;i<count;i++{
    		go foo(i)
    	}
    	for i:=0;i<count;i++{
    		<- quit
    	}
    }
    // 无缓冲的信道是一批数据一个一个的流进流出，处理是乱序的

    package main
    import "fmt"
    func foo(id int){
    	fmt.Println(id)
    }
    func main(){
    	count := 1000
    	quit := make(chan int,1000)
    	for i:=0;i<count;i++{
    		foo(i)
    		quit <- i
    	}
    	for i:=0;i<count;i++{
    		<- quit
    	}
    }
    // 有缓冲的信道是数据一个一个进行处理，处理是顺序的