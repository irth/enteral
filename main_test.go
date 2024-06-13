package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestIsShort(t *testing.T) {
	app := App{cache: DummyCache{}}

	s, err := app.IsShort(context.Background(), "3aGylFJAzTk")
	assert.NoError(t, err)
	assert.True(t, s, fmt.Sprintf("https://youtu.be/3aGylFJAzTk"))

	s, err = app.IsShort(context.Background(), "UrxkO4DM67M")
	assert.NoError(t, err)
	assert.False(t, s, fmt.Sprintf("https://youtu.be/UrxkO4DM67M"))
}

func TestValkeyCache(t *testing.T) {
	cache, err := NewValkeyIsShortCache("redis://127.0.0.1:6379")
	assert.NoError(t, err)

	err = cache.Set(context.Background(), "other", "nie-istnieje", time.Second*5, "1")
	assert.NoError(t, err)

	_, err = cache.Get(context.Background(), "key", "nie-istnieje")
	assert.Equal(t, err, KeyNotFound)

	err = cache.Set(context.Background(), "key", "istnieje", time.Second*5, "1")
	assert.NoError(t, err)

	v, err := cache.Get(context.Background(), "key", "istnieje")
	assert.NoError(t, err)
	assert.Equal(t, v, "1")

	err = cache.Set(context.Background(), "key", "istnieje", time.Second*5, "0")
	assert.NoError(t, err)

	v, err = cache.Get(context.Background(), "key", "istnieje")
	assert.NoError(t, err)
	assert.Equal(t, v, "0")

	err = cache.Set(context.Background(), "key", "expires", time.Millisecond*100, "0")
	assert.NoError(t, err)

	time.Sleep(time.Millisecond * 200)
	_, err = cache.Get(context.Background(), "key", "expires")
	assert.Equal(t, err, KeyNotFound)
}
