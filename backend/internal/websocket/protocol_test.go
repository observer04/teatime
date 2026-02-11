package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NewMessage Tests
// =============================================================================

func TestNewMessage_CreatesCorrectEnvelope(t *testing.T) {
	before := time.Now()
	msg, err := NewMessage("test.event", map[string]string{"key": "value"})
	after := time.Now()

	require.NoError(t, err)
	require.NotNil(t, msg)

	assert.Equal(t, "test.event", msg.Type)
	assert.NotNil(t, msg.Payload)
	assert.False(t, msg.Timestamp.IsZero())
	assert.True(t, !msg.Timestamp.Before(before) && !msg.Timestamp.After(after))
}

func TestNewMessage_NilPayload(t *testing.T) {
	msg, err := NewMessage("test.event", nil)
	require.NoError(t, err)
	assert.Equal(t, json.RawMessage("null"), msg.Payload)
}

func TestNewMessage_InvalidPayload(t *testing.T) {
	// Channels cannot be marshalled to JSON
	msg, err := NewMessage("test.event", make(chan int))
	assert.Error(t, err)
	assert.Nil(t, msg)
}

func TestNewMessage_JSONSerialization(t *testing.T) {
	msg, err := NewMessage(EventTypeMessageNew, MessageNewPayload{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		SenderID:       uuid.New(),
		SenderUsername: "alice",
		BodyText:       "Hello!",
		CreatedAt:      time.Now(),
	})
	require.NoError(t, err)

	// Verify the whole message serializes cleanly
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, EventTypeMessageNew, decoded.Type)
	assert.NotEmpty(t, decoded.Payload)
}

// =============================================================================
// Payload Round-Trip Tests
// =============================================================================

func TestAuthPayload_RoundTrip(t *testing.T) {
	original := AuthPayload{Token: "jwt-token-123"}
	data, _ := json.Marshal(original)
	var decoded AuthPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.Token, decoded.Token)
}

func TestRoomJoinPayload_RoundTrip(t *testing.T) {
	id := uuid.New().String()
	original := RoomJoinPayload{ConversationID: id}
	data, _ := json.Marshal(original)
	var decoded RoomJoinPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, id, decoded.ConversationID)
}

func TestMessageSendPayload_RoundTrip(t *testing.T) {
	original := MessageSendPayload{
		ConversationID: uuid.New().String(),
		BodyText:       "Hello world!",
		AttachmentID:   uuid.New().String(),
		TempID:         "temp-123",
	}
	data, _ := json.Marshal(original)
	var decoded MessageSendPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestTypingPayload_RoundTrip(t *testing.T) {
	original := TypingPayload{ConversationID: uuid.New().String()}
	data, _ := json.Marshal(original)
	var decoded TypingPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestReceiptReadPayload_RoundTrip(t *testing.T) {
	original := ReceiptReadPayload{MessageID: uuid.New().String()}
	data, _ := json.Marshal(original)
	var decoded ReceiptReadPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestErrorPayload_RoundTrip(t *testing.T) {
	original := ErrorPayload{Code: "forbidden", Message: "Access denied"}
	data, _ := json.Marshal(original)
	var decoded ErrorPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestAuthSuccessPayload_RoundTrip(t *testing.T) {
	original := AuthSuccessPayload{
		UserID:   uuid.New(),
		Username: "alice",
	}
	data, _ := json.Marshal(original)
	var decoded AuthSuccessPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestMessageNewPayload_RoundTrip(t *testing.T) {
	attachmentID := uuid.New()
	original := MessageNewPayload{
		ID:             uuid.New(),
		ConversationID: uuid.New(),
		SenderID:       uuid.New(),
		SenderUsername: "alice",
		BodyText:       "Test message",
		AttachmentID:   &attachmentID,
		CreatedAt:      time.Now().Truncate(time.Millisecond),
		TempID:         "temp-abc",
	}
	data, _ := json.Marshal(original)
	var decoded MessageNewPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.ID, decoded.ID)
	assert.Equal(t, original.SenderUsername, decoded.SenderUsername)
	assert.Equal(t, original.BodyText, decoded.BodyText)
	assert.Equal(t, original.TempID, decoded.TempID)
	require.NotNil(t, decoded.AttachmentID)
	assert.Equal(t, *original.AttachmentID, *decoded.AttachmentID)
}

func TestTypingBroadcastPayload_RoundTrip(t *testing.T) {
	original := TypingBroadcastPayload{
		ConversationID: uuid.New(),
		UserID:         uuid.New(),
		Username:       "bob",
		IsTyping:       true,
	}
	data, _ := json.Marshal(original)
	var decoded TypingBroadcastPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestPresencePayload_RoundTrip(t *testing.T) {
	original := PresencePayload{
		UserID:   uuid.New(),
		Username: "alice",
		Online:   true,
	}
	data, _ := json.Marshal(original)
	var decoded PresencePayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestMemberJoinedPayload_RoundTrip(t *testing.T) {
	original := MemberJoinedPayload{
		ConversationID: uuid.New(),
		UserID:         uuid.New(),
		Username:       "charlie",
		Role:           "member",
		AddedBy:        uuid.New(),
	}
	data, _ := json.Marshal(original)
	var decoded MemberJoinedPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestMemberLeftPayload_RoundTrip(t *testing.T) {
	original := MemberLeftPayload{
		ConversationID: uuid.New(),
		UserID:         uuid.New(),
		Username:       "bob",
		RemovedBy:      uuid.New(),
	}
	data, _ := json.Marshal(original)
	var decoded MemberLeftPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestRoomUpdatedPayload_RoundTrip(t *testing.T) {
	original := RoomUpdatedPayload{
		ConversationID: uuid.New(),
		Title:          "New Group Name",
		UpdatedBy:      uuid.New(),
	}
	data, _ := json.Marshal(original)
	var decoded RoomUpdatedPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestMessageDeletedPayload_RoundTrip(t *testing.T) {
	original := MessageDeletedPayload{
		MessageID:      uuid.New(),
		ConversationID: uuid.New(),
		DeletedBy:      uuid.New(),
	}
	data, _ := json.Marshal(original)
	var decoded MessageDeletedPayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}

func TestReceiptUpdatePayload_RoundTrip(t *testing.T) {
	original := ReceiptUpdatePayload{
		MessageID:      uuid.New(),
		ConversationID: uuid.New(),
		UserID:         uuid.New(),
		Status:         "read",
		Timestamp:      time.Now().Truncate(time.Millisecond),
	}
	data, _ := json.Marshal(original)
	var decoded ReceiptUpdatePayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.Status, decoded.Status)
	assert.Equal(t, original.MessageID, decoded.MessageID)
}

func TestReceiptBatchUpdatePayload_RoundTrip(t *testing.T) {
	original := ReceiptBatchUpdatePayload{
		ConversationID: uuid.New(),
		MessageIDs:     []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		UserID:         uuid.New(),
		Status:         "delivered",
		Timestamp:      time.Now().Truncate(time.Millisecond),
	}
	data, _ := json.Marshal(original)
	var decoded ReceiptBatchUpdatePayload
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original.Status, decoded.Status)
	assert.Len(t, decoded.MessageIDs, 3)
}

// =============================================================================
// Message Envelope JSON Format Tests
// =============================================================================

func TestMessage_JSONFormat(t *testing.T) {
	msg, _ := NewMessage("test.event", map[string]string{"hello": "world"})
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	// Verify JSON structure
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal(data, &raw))

	assert.Contains(t, raw, "type")
	assert.Contains(t, raw, "payload")
	assert.Contains(t, raw, "timestamp")
	assert.Equal(t, "test.event", raw["type"])
}

func TestMessage_EmptyPayload(t *testing.T) {
	msg := &Message{
		Type:      "test.ping",
		Timestamp: time.Now(),
	}
	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded Message
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "test.ping", decoded.Type)
}

// =============================================================================
// Event Type Constants Tests
// =============================================================================

func TestEventTypeConstants_NotEmpty(t *testing.T) {
	// Verify all event types are non-empty strings
	clientEvents := []string{
		EventTypeAuth, EventTypeRoomJoin, EventTypeRoomLeave,
		EventTypeMessageSend, EventTypeTypingStart, EventTypeTypingStop,
		EventTypeReceiptRead,
	}
	for _, e := range clientEvents {
		assert.NotEmpty(t, e, "client event type should not be empty")
	}

	serverEvents := []string{
		EventTypeError, EventTypeAuthSuccess, EventTypeMessageNew,
		EventTypeMessageDeleted, EventTypeTyping, EventTypeReceiptUpdate,
		EventTypeMemberJoined, EventTypeMemberLeft, EventTypeRoomUpdated,
		EventTypePresence,
	}
	for _, e := range serverEvents {
		assert.NotEmpty(t, e, "server event type should not be empty")
	}
}
