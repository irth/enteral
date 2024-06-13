package main

import (
	"context"
	"fmt"
	"time"

	"github.com/valkey-io/valkey-go"
)

var KeyNotFound = fmt.Errorf("key not found")

type Cache interface {
	Get(ctx context.Context, key string, id string) (string, error)
	Set(ctx context.Context, key string, id string, expiry time.Duration, v string) error
}

type DummyCache struct{}

func (d DummyCache) Get(ctx context.Context, key string, id string) (string, error) {
	return "", KeyNotFound
}

func (d DummyCache) Set(ctx context.Context, key string, id string, expiry time.Duration, feed string) error {
	return nil
}

type ValkeyCache struct {
	client valkey.Client
}

func NewValkeyIsShortCache(connUrl string) (Cache, error) {
	opt, err := valkey.ParseURL(connUrl)
	if err != nil {
		return nil, err
	}

	client, err := valkey.NewClient(opt)
	if err != nil {
		return nil, err
	}

	return ValkeyCache{client}, nil
}

func (r ValkeyCache) key(key string, id string) string {
	return fmt.Sprintf("enteral-cache:%s:%s", key, id)
}

func (r ValkeyCache) Get(ctx context.Context, key string, id string) (string, error) {
	key = r.key(key, id)
	v, err := r.client.Do(ctx, r.client.B().Get().Key(key).Build()).ToString()
	if err != nil {
		if err == valkey.Nil {
			return "", KeyNotFound
		}
		return "", err
	}
	return v, nil
}

func (r ValkeyCache) Set(ctx context.Context, key string, id string, expiry time.Duration, v string) error {
	key = r.key(key, id)

	return r.client.Do(ctx, r.client.B().Set().Key(key).Value(v).Px(expiry).Build()).Error()
}
