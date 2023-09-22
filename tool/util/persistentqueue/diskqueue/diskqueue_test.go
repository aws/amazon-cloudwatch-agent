package diskqueue

import (
	"encoding/json"
	"errors"
	"github.com/aws/amazon-cloudwatch-agent/tool/util/persistentqueue"
	"github.com/influxdata/telegraf/plugins/common/shim"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDiskQueue(
	t *testing.T,
	marshaler func(interface{}) ([]byte, error),
	unmarshaler func([]byte) (interface{}, error),
) persistentqueue.PersistentQueue {
	return NewPersistentQueue(
		"test",
		t.TempDir(),
		2,
		1024*1024,
		1,
		100*1024,
		5,
		time.Minute,
		marshaler,
		unmarshaler,
		shim.NewLogger(),
	)
}

type testStruct struct {
	M string
}

func TestEnqueueDequeue(t *testing.T) {
	marshaler := func(i interface{}) ([]byte, error) {
		return json.Marshal(i)
	}
	unmarshaler := func(bytes []byte) (interface{}, error) {
		var obj testStruct
		return obj, json.Unmarshal(bytes, &obj)
	}
	queue := setupDiskQueue(t, marshaler, unmarshaler)
	assert.Zero(t, queue.Depth())

	expected := testStruct{M: "msg"}
	require.NoError(t, queue.Enqueue(expected))
	assert.Equal(t, int64(1), queue.Depth())

	actual, err := queue.Dequeue()
	require.NoError(t, err)
	assert.Zero(t, queue.Depth())
	assert.Equal(t, expected, actual.(testStruct))
	assert.NoError(t, queue.Close())
}

func TestEnqueueError(t *testing.T) {
	marshaler := func(i interface{}) ([]byte, error) {
		return nil, errors.New("")
	}
	unmarshaler := func(bytes []byte) (interface{}, error) {
		return struct{}{}, nil
	}
	queue := setupDiskQueue(t, marshaler, unmarshaler)

	assert.Error(t, queue.Enqueue(struct{}{}))
	assert.NoError(t, queue.Close())
}

func TestDequeueError(t *testing.T) {
	marshaler := func(i interface{}) ([]byte, error) {
		return []byte("test"), nil
	}
	unmarshaler := func(bytes []byte) (interface{}, error) {
		return nil, errors.New("")
	}
	queue := setupDiskQueue(t, marshaler, unmarshaler)

	assert.NoError(t, queue.Enqueue(struct{}{}))
	_, err := queue.Dequeue()
	assert.Error(t, err)
	assert.NoError(t, queue.Close())
}

func TestSaturatedEnqueueDequeue(t *testing.T) {
	marshaler := func(i interface{}) ([]byte, error) {
		return json.Marshal(i)
	}
	unmarshaler := func(bytes []byte) (interface{}, error) {
		var obj testStruct
		return obj, json.Unmarshal(bytes, &obj)
	}
	queue := setupDiskQueue(t, marshaler, unmarshaler)

	expectedStruct1 := testStruct{M: "msg"}
	expectedStruct2 := testStruct{M: "msg"}
	// saturate the queue (of size 2) so that the first element is dropped
	require.NoError(t, queue.Enqueue(testStruct{M: "msg"}))
	require.NoError(t, queue.Enqueue(expectedStruct1))
	require.NoError(t, queue.Enqueue(expectedStruct2))
	assert.Equal(t, int64(2), queue.Depth())

	actualStruct1, err := queue.Dequeue()
	require.NoError(t, err)

	actualStruct2, err := queue.Dequeue()
	require.NoError(t, err)

	assert.Equal(t, expectedStruct1, actualStruct1.(testStruct))
	assert.Equal(t, expectedStruct2, actualStruct2.(testStruct))
	assert.NoError(t, queue.Close())
}
