package database

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCursorEncoder(t *testing.T) {
	encoder := NewCursorEncoder()

	t.Run("Task Cursor Encoding/Decoding", func(t *testing.T) {
		// Create a test cursor
		originalCursor := TaskCursor{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC().Truncate(time.Microsecond), // Truncate to handle precision
			Priority:  intPtr(5),
		}

		// Encode the cursor
		encoded, err := encoder.EncodeTaskCursor(originalCursor)
		require.NoError(t, err)
		assert.NotEmpty(t, encoded)

		// Decode the cursor
		decodedCursor, err := encoder.DecodeTaskCursor(encoded)
		require.NoError(t, err)

		// Verify the decoded cursor matches the original
		assert.Equal(t, originalCursor.ID, decodedCursor.ID)
		assert.Equal(t, originalCursor.CreatedAt, decodedCursor.CreatedAt)
		assert.Equal(t, *originalCursor.Priority, *decodedCursor.Priority)
	})

	t.Run("Execution Cursor Encoding/Decoding", func(t *testing.T) {
		// Create a test cursor
		originalCursor := ExecutionCursor{
			ID:        uuid.New(),
			CreatedAt: time.Now().UTC().Truncate(time.Microsecond), // Truncate to handle precision
		}

		// Encode the cursor
		encoded, err := encoder.EncodeExecutionCursor(originalCursor)
		require.NoError(t, err)
		assert.NotEmpty(t, encoded)

		// Decode the cursor
		decodedCursor, err := encoder.DecodeExecutionCursor(encoded)
		require.NoError(t, err)

		// Verify the decoded cursor matches the original
		assert.Equal(t, originalCursor.ID, decodedCursor.ID)
		assert.Equal(t, originalCursor.CreatedAt, decodedCursor.CreatedAt)
	})

	t.Run("Invalid Cursor Handling", func(t *testing.T) {
		// Test empty cursor
		_, err := encoder.DecodeTaskCursor("")
		assert.Equal(t, ErrInvalidCursor, err)

		// Test invalid base64
		_, err = encoder.DecodeTaskCursor("invalid-base64!")
		assert.Error(t, err)

		// Test invalid JSON
		_, err = encoder.DecodeTaskCursor("aW52YWxpZC1qc29u") // "invalid-json" in base64
		assert.Error(t, err)
	})
}

func TestValidatePaginationRequest(t *testing.T) {
	t.Run("Default Values", func(t *testing.T) {
		req := &CursorPaginationRequest{}
		ValidatePaginationRequest(req)

		assert.Equal(t, 20, req.Limit)
		assert.Equal(t, "desc", req.SortOrder)
	})

	t.Run("Limit Capping", func(t *testing.T) {
		req := &CursorPaginationRequest{
			Limit: 200, // Above max
		}
		ValidatePaginationRequest(req)

		assert.Equal(t, 100, req.Limit) // Should be capped
	})

	t.Run("Sort Order Validation", func(t *testing.T) {
		req := &CursorPaginationRequest{
			SortOrder: "invalid",
		}
		ValidatePaginationRequest(req)

		assert.Equal(t, "desc", req.SortOrder) // Should default to desc
	})
}

func TestBuildTaskCursorWhere(t *testing.T) {
	userID := uuid.New()
	status := "pending"
	cursor := &TaskCursor{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
	}

	t.Run("With User ID and Status", func(t *testing.T) {
		whereClause, args := BuildTaskCursorWhere(cursor, "desc", &userID, &status)
		
		assert.Contains(t, whereClause, "WHERE")
		assert.Contains(t, whereClause, "user_id")
		assert.Contains(t, whereClause, "status")
		assert.Contains(t, whereClause, "created_at <")
		assert.Len(t, args, 5) // userID, status, cursor.CreatedAt (2x), cursor.ID
	})

	t.Run("Without Cursor", func(t *testing.T) {
		whereClause, args := BuildTaskCursorWhere(nil, "desc", &userID, &status)
		
		assert.Contains(t, whereClause, "WHERE")
		assert.Contains(t, whereClause, "user_id")
		assert.Contains(t, whereClause, "status")
		assert.NotContains(t, whereClause, "created_at")
		assert.Len(t, args, 2) // userID, status
	})

	t.Run("Ascending Order", func(t *testing.T) {
		whereClause, args := BuildTaskCursorWhere(cursor, "asc", nil, nil)
		
		assert.Contains(t, whereClause, "created_at >") // Should use > for asc
		assert.Len(t, args, 3) // cursor.CreatedAt (2x), cursor.ID
	})
}

