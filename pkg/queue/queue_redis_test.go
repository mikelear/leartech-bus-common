package queue

import (
	"encoding/json"
	"testing"

	rediscommon "github.com/mikelear/leartech-bus-common/pkg/redis"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisQueue_PushAndPop(t *testing.T) {
	mockRedis := rediscommon.NewMockRedisClient(t)

	ctx := t.Context()
	queue := "Queue"
	objToQueue := TestStruct{
		Name: "ObjectToQueue",
	}
	objBytes, err := json.Marshal(objToQueue)
	require.NoError(t, err)

	c := NewRedisQueue[TestStruct](mockRedis)

	mockRedis.On("LPush", ctx, queue, []interface{}{string(objBytes)}).Return(redis.NewIntResult(1, nil))
	err = c.Push(ctx, queue, objToQueue)
	require.NoError(t, err)

	mockRedis.On("RPop", ctx, queue).Return(redis.NewStringResult(string(objBytes), nil))
	out, err := c.Pop(ctx, queue)
	require.NoError(t, err)
	assert.Equal(t, objToQueue, out)
}

type TestStruct struct {
	Name string `json:"name"`
}
