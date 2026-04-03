//go:build integration

package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/redis/go-redis/v9"
)

func redisAddr(t *testing.T) string {
	t.Helper()
	u := os.Getenv("REDIS_URL")
	if u == "" {
		u = "redis://127.0.0.1:6379"
	}
	return u
}

func natsURL(t *testing.T) string {
	t.Helper()
	u := getenvDefault("NATS_URL", "nats://127.0.0.1:4222")
	return u
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestRedisConnectivity(t *testing.T) {
	opts, err := redis.ParseURL(redisAddr(t))
	if err != nil {
		t.Fatalf("parse redis url: %v", err)
	}
	c := redis.NewClient(opts)
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	for {
		if err := c.Ping(ctx).Err(); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("redis ping: %v", ctx.Err())
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func TestNATSConnectivity(t *testing.T) {
	var nc *nats.Conn
	var err error
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		nc, err = nats.Connect(natsURL(t), nats.Timeout(2*time.Second), nats.MaxReconnects(0))
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("nats connect: %v", err)
	}
	t.Cleanup(nc.Close)

	if !nc.IsConnected() {
		t.Fatal("expected connected NATS client")
	}
}
