package main

import (
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

func acquireLock(client *redis.Client, lockKey string, expiration time.Duration) (bool, error) {
	// Try to acquire the lock
	result, err := client.SetNX(client.Context(), lockKey, "locked", expiration).Result()
	if err != nil {
		return false, err
	}

	return result, nil
}


func releaseLock(client *redis.Client, lockKey string) error {
	// Release the lock by deleting the key
	_, err := client.Del(client.Context(), lockKey).Result()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	// Create a Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // Set your Redis password here
		DB:       0,
	})

	// Acquire the lock
	lockKey := "my_lock"
	expiration := 10 * time.Second
	acquired, err := acquireLock(client, lockKey, expiration)
	if err != nil {
		fmt.Println("Failed to acquire lock:", err)
		return
	}

	if acquired {
		fmt.Println("Lock acquired successfully")
		// Do your critical section here

		// Release the lock
		err := releaseLock(client, lockKey)
		if err != nil {
			fmt.Println("Failed to release lock:", err)
			return
		}

		fmt.Println("Lock released successfully")
	} else {
		fmt.Println("Failed to acquire lock")
	}
}
// Output:
// Lock acquired successfully
// Lock released successfully
