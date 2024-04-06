import (
    "context"
    "time"

    "github.com/go-redis/redis/v8"
)
// 如果在锁到期后任务还没有处理完，可能会导致其他进程或线程获取到锁并开始执行任务，这可能会导致数据不一致或其他并发问题。

// 为了解决这个问题，你可以使用以下几种策略：

// 锁续期：如果你预计任务可能会超过锁的过期时间，你可以在一个单独的线程或协程中定期检查任务是否仍在运行，如果是，则自动续期锁。这需要使用 Redis 的 EXPIRE 命令来更新锁的过期时间。

// 锁超时：设置一个足够长的锁超时时间，以确保任务在锁过期之前完成。但是，这种方法可能会导致资源被长时间锁定，如果任务失败或者忘记释放锁，可能会导致死锁。

// 可重入锁：如果任务可能会被同一线程多次获取，可以使用可重入锁。这种锁允许同一线程多次获取，而不会导致阻塞。

// 检查并设置：在执行任务前检查锁是否仍然被当前线程持有，如果不是，则不执行任务。这需要在 Redis 中存储一个唯一的锁标识（例如，UUID 或线程 ID），并在获取锁时检查这个标识。

// 以上策略的选择取决于你的具体需求和环境。
// 在 Go 中实现 Redis 分布式锁的续期，可以通过在单独的 goroutine 中定期检查并更新锁的过期时间来实现。以下是一个简单的示例：

type RedisLock struct {
    client *redis.Client
    lockKey string
    expiration time.Duration
    stopRenew chan struct{}
}

func NewRedisLock(client *redis.Client, lockKey string, expiration time.Duration) *RedisLock {
    return &RedisLock{
        client: client,
        lockKey: lockKey,
        expiration: expiration,
        stopRenew: make(chan struct{}),
    }
}
// SETNX 确保原子性
func (r *RedisLock) AcquireLock() (bool, error) {
    result, err := r.client.SetNX(context.Background(), r.lockKey, "locked", r.expiration).Result()
    if err != nil {
        return false, err
    }

    if result {
        go r.renewLock()
    }

    return result, nil
}
//Lua脚本方式保证原子性
func (r *RedisLock) AcquireLock2() (bool, error) {
    script := redis.NewScript(`
        if redis.call("setnx", KEYS[1], ARGV[1]) == 1 then
            redis.call("expire", KEYS[1], ARGV[2])
            return 1
        else
            return 0
        end
    `)

    result, err := script.Run(context.Background(), r.client, []string{r.lockKey}, "locked", r.expiration.Seconds()).Result()
    if err != nil {
        return false, err
    }

    return result.(int64) == 1, nil
}

func (r *RedisLock) ReleaseLock() error {
    r.stopRenew <- struct{}{}
    _, err := r.client.Del(context.Background(), r.lockKey).Result()
    if err != nil {
        return err
    }
    return nil
}

func (r *RedisLock) renewLock() {
    ticker := time.NewTicker(r.expiration / 2)
    defer ticker.Stop()

    for {
        select {
        case <-r.stopRenew:
            return
        case <-ticker.C:
            r.client.Expire(context.Background(), r.lockKey, r.expiration)
        }
    }
}

func main() {
    // 创建一个 Redis 客户端
    client := redis.NewClient(&redis.Options{
        Addr:     "localhost:6379",
        Password: "", // no password set
        DB:       0,  // use default DB
    })

    // 锁的键名
    lockKey := "myLock"

    // 创建一个 RedisLock 实例
    lock := NewRedisLock(client, lockKey, 10*time.Second)

    // 模拟并发请求
    for i := 0; i < 10; i++ {
        go func(i int) {
            // 尝试获取锁
            acquired, err := lock.AcquireLock()
            if err != nil {
                fmt.Printf("Error while acquiring lock: %v\n", err)
                return
            }

            if acquired {
                fmt.Printf("Lock acquired by request %d, do work here\n", i)

                // 执行需要并发控制的代码...

                // 释放锁
                err = lock.ReleaseLock()
                if err != nil {
                    fmt.Printf("Error while releasing lock: %v\n", err)
                }
            } else {
                fmt.Printf("Could not acquire lock for request %d\n", i)
            }
        }(i)
    }

    // 等待所有请求完成
    time.Sleep(20 * time.Second)
}
//output
// Lock acquired by request 0, do work here
// Lock acquired by request 1, do work here
// Lock acquired by request 2, do work here
// Lock acquired by request 3, do work here
// Lock acquired by request 4, do work here
// Lock acquired by request 5, do work here
// Lock acquired by request 6, do work here
// Lock acquired by request 7, do work here
// Lock acquired by request 8, do work here
// Lock acquired by request 9, do work here
// 通过在单独的 goroutine 中定期更新锁的过期时间，可以实现 Redis 分布式锁的续期功能。这样可以确保任务在锁过期之前完成，而不会被其他进程或线程中断。在上面的示例中，我们创建了一个 RedisLock 结构体，其中包含了 Redis 客户端、锁的键名、过期时间和一个用于停止续期的通道。AcquireLock 方法用于获取锁，并在成功获取锁后启动一个 goroutine 来定期更新锁的过期时间。ReleaseLock 方法用于释放锁，并停止续期 goroutine。在主函数中，我们模拟了 10 个并发请求，每个请求尝试获取锁并执行一些需要并发控制的代码。通过这种方式，我们可以确保任务在锁过期之前完成，并避免并发问题。