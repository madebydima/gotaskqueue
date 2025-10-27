package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	fmt.Println("Waiting for Redis to be ready...")

	for i := 0; i < 30; i++ {
		err := client.Ping(ctx).Err()
		if err == nil {
			fmt.Println("Redis is ready!")
			return
		}

		fmt.Printf("Attempt %d: Redis not ready yet: %v\n", i+1, err)
		time.Sleep(1 * time.Second)
	}

	log.Fatal("Redis failed to become ready within 30 seconds")
}
