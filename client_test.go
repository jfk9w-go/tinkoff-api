package tinkoff_test

import (
	"encoding/json"
	"testing"

	"github.com/jkf9w-go/tinkoff-api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTest(t *testing.T) {
	str := `{"account": "123"}`
	var operation tinkoff.Operation
	err := json.Unmarshal([]byte(str), &operation)
	require.NoError(t, err)
	assert.Equal(t, "123", operation.Account)
}
