import (
    "fmt"
    "sync"
    "time"

    "github.com/go-redis/redis/v8"
)
// 在 Go 中实现 Redis 的可重入锁，需要在 Redis 中存储一个唯一的锁标识（例如，UUID 或线程 ID），并在获取锁时检查这个标识。
type RedisReentrantLock struct {
    client     *redis.Client
    lockKey    string
    identifier string
    expiration time.Duration
    mutex      sync.Mutex
    lockCount  int
}

func NewRedisReentrantLock(client *redis.Client, lockKey string, identifier string, expiration time.Duration) *RedisReentrantLock {
    return &RedisReentrantLock{
        client:     client,
        lockKey:    lockKey,
        identifier: identifier,
        expiration: expiration,
    }
}

func (r *RedisReentrantLock) AcquireLock() (bool, error) {
    r.mutex.Lock()
    defer r.mutex.Unlock()

    if r.lockCount > 0 {
        r.lockCount++
        return true, nil
    }

    ok, err := r.client.SetNX(context.Background(), r.lockKey, r.identifier, r.expiration).Result()
    if err != nil {
        return false, err
    }

    if ok {
        r.lockCount++
    }

    return ok, nil
}

func (r *RedisReentrantLock) ReleaseLock() error {
    r.mutex.Lock()
    defer r.mutex.Unlock()

    if r.lockCount > 1 {
        r.lockCount--
        return nil
    }

    if r.lockCount <= 0 {
        return fmt.Errorf("no lock acquired")
    }

    val, err := r.client.Get(context.Background(), r.lockKey).Result()
    if err != nil {
        return err
    }

    if val != r.identifier {
        return fmt.Errorf("lock identifier mismatch")
    }

    r.lockCount--
	// 如果此时锁到期自动释放了，其他获得了锁，下面的释放锁会报错，无法保证删除的原子性
	//方法一：r中有锁保护，r.mutex.Lock()，但是这样会导致锁的粒度变大，性能下降
	//方法二：使用lua脚本保证原子性
// 	script := `
// 	if redis.call("GET", KEYS[1]) == ARGV[1] then
// 		return redis.call("DEL", KEYS[1])
// 	else
// 		return 0
// 	end
// `

// 	_, err = r.client.Eval(context.Background(), script, []string{r.lockKey}, r.identifier).Result()
// 	if err != nil {
// 		return err
// 	}
	//方法三：使用watch监视key，如果key的值没有变化，就删除key
	// 使用watch监视key，如果key的值没有变化，就删除key
	// _, err = r.client.Watch(context.Background(), func(tx *redis.Tx) error {
	// 	val, err := tx.Get(context.Background(), r.lockKey).Result()
	// 	if err != nil {
	// 		return err
	// 	}

	// 	if val == r.identifier {
	// 		return tx.Del(context.Background(), r.lockKey).Err()
	// 	}

	// 	return nil
	// })
	// if err != nil {
	// 	return err
	// }
	
	// 释放锁
    return r.client.Del(context.Background(), r.lockKey).Err()
}
// 在这个示例中，我们首先创建了一个 RedisReentrantLock 结构体，它包含了一个 Redis 客户端、一个锁的键名、一个唯一的锁标识、一个锁的过期时间、一个互斥锁和一个锁计数器。然后我们定义了 AcquireLock 和 ReleaseLock 方法。在 AcquireLock 方法中，我们首先检查锁计数器，如果锁计数器大于 0，说明当前线程已经获取了锁，我们只需要增加锁计数器的值，然后返回 true。如果锁计数器等于 0，说明当前线程还没有获取锁，我们需要使用 SETNX 命令尝试获取锁。在 ReleaseLock 方法中，我们首先检查锁计数器，如果锁计数器大于 1，说明当前线程还需要持有锁，我们只需要减少锁计数器的值，然后返回 nil。如果锁计数器等于 1，说明当前线程可以释放锁，我们需要检查 Redis 中的锁标识，如果锁标识和当前线程的锁标识匹配，我们就删除锁。

