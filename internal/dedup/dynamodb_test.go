package dedup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// fakeDynamoDBAPI is a hand-rolled DynamoDBAPI implementation that captures the
// last call and returns the configured error.
type fakeDynamoDBAPI struct {
	gotInput    *dynamodb.PutItemInput
	err         error
	calls       int
	gotDelete   *dynamodb.DeleteItemInput
	deleteErr   error
	deleteCalls int
}

func (f *fakeDynamoDBAPI) PutItem(_ context.Context, in *dynamodb.PutItemInput,
	_ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	f.calls++
	f.gotInput = in
	if f.err != nil {
		return nil, f.err
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeDynamoDBAPI) DeleteItem(_ context.Context, in *dynamodb.DeleteItemInput,
	_ ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error) {
	f.deleteCalls++
	f.gotDelete = in
	if f.deleteErr != nil {
		return nil, f.deleteErr
	}
	return &dynamodb.DeleteItemOutput{}, nil
}

func TestDynamoDB_Disabled_ReturnsTrue(t *testing.T) {
	// nil receiver, empty table, nil client — all three branches.
	cases := []struct {
		name  string
		store *DynamoDBStore
	}{
		{name: "nil-receiver", store: nil},
		{name: "empty-table", store: &DynamoDBStore{}},
		{name: "nil-client", store: &DynamoDBStore{TableName: "t"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.store.TryReserve(context.Background(), "key", nil)
			if err != nil {
				t.Fatalf("TryReserve: %v", err)
			}
			if !got {
				t.Fatal("disabled store should fail open and return true")
			}
		})
	}
}

func TestDynamoDB_FirstSeen_ReturnsTrue(t *testing.T) {
	api := &fakeDynamoDBAPI{}
	store := NewDynamoDBWithClient(api, "test-table", 24*time.Hour)
	got, err := store.TryReserve(context.Background(), "key-1", map[string]string{"finding_arn": "arn:x"})
	if err != nil {
		t.Fatalf("TryReserve: %v", err)
	}
	if !got {
		t.Fatal("first sighting should return true")
	}
	if api.calls != 1 {
		t.Fatalf("PutItem calls = %d, want 1", api.calls)
	}
	if api.gotInput.ConditionExpression == nil || *api.gotInput.ConditionExpression != "attribute_not_exists(dedup_key)" {
		t.Fatalf("condition = %v, want attribute_not_exists(dedup_key)",
			api.gotInput.ConditionExpression)
	}
}

// TestDynamoDB_ExpireAt_IsNumberAttribute verifies the TTL value is
// persisted as a DynamoDB N attribute, not S, so the TTL scanner recognizes
// it.
func TestDynamoDB_ExpireAt_IsNumberAttribute(t *testing.T) {
	api := &fakeDynamoDBAPI{}
	store := NewDynamoDBWithClient(api, "test-table", 7*24*time.Hour)
	if _, err := store.TryReserve(context.Background(), "key", nil); err != nil {
		t.Fatalf("TryReserve: %v", err)
	}
	got, ok := api.gotInput.Item[attrExpireAt]
	if !ok {
		t.Fatalf("expire_at not present in marshaled item: %+v", api.gotInput.Item)
	}
	if _, isN := got.(*types.AttributeValueMemberN); !isN {
		t.Fatalf("expire_at type = %T, want *types.AttributeValueMemberN (DynamoDB N)", got)
	}
	if _, isS := got.(*types.AttributeValueMemberS); isS {
		t.Fatalf("expire_at must NOT be a string attribute (S)")
	}
}

// TestDynamoDB_ConditionalCheckFailed_ReturnsFalse verifies the store
// uses errors.As to detect the conditional-check exception and returns
// (false, nil).
func TestDynamoDB_ConditionalCheckFailed_ReturnsFalse(t *testing.T) {
	api := &fakeDynamoDBAPI{err: &types.ConditionalCheckFailedException{Message: stringPtr("exists")}}
	store := NewDynamoDBWithClient(api, "t", time.Hour)
	got, err := store.TryReserve(context.Background(), "key", nil)
	if err != nil {
		t.Fatalf("TryReserve: %v", err)
	}
	if got {
		t.Fatal("conditional check failed should return false (key already exists)")
	}
}

// TestDynamoDB_ConditionalCheckFailed_WrappedError covers the wrapped-error
// branch — errors.As must unwrap through fmt.Errorf %w.
func TestDynamoDB_ConditionalCheckFailed_WrappedError(t *testing.T) {
	api := &fakeDynamoDBAPI{err: wrap("smithy operation error",
		&types.ConditionalCheckFailedException{Message: stringPtr("exists")})}
	store := NewDynamoDBWithClient(api, "t", time.Hour)
	got, err := store.TryReserve(context.Background(), "key", nil)
	if err != nil {
		t.Fatalf("TryReserve: %v", err)
	}
	if got {
		t.Fatal("wrapped conditional check failed should still be detected")
	}
}

// TestDynamoDB_OtherError_FailsOpen covers the fail-open contract: any
// non-ConditionalCheck error logs and returns (true, nil).
func TestDynamoDB_OtherError_FailsOpen(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := slog.New(slog.NewJSONHandler(buf, nil))
	api := &fakeDynamoDBAPI{err: errors.New("RequestLimitExceeded")}
	store := NewDynamoDBWithClient(api, "t", time.Hour).WithLogger(logger)
	got, err := store.TryReserve(context.Background(), "key", nil)
	if err != nil {
		t.Fatalf("TryReserve: %v", err)
	}
	if !got {
		t.Fatal("transient SDK error should fail open and return true")
	}
	var line map[string]any
	if jerr := json.Unmarshal(stripFirstLine(buf.Bytes()), &line); jerr != nil {
		t.Fatalf("logger output not JSON: %v\n%s", jerr, buf.String())
	}
	if level, _ := line["level"].(string); level != "WARN" {
		t.Fatalf("log level = %q, want WARN", level)
	}
}

// TestDynamoDB_MetadataMerged ensures user metadata is merged into the item
// (without overriding the reserved attribute names).
func TestDynamoDB_MetadataMerged(t *testing.T) {
	api := &fakeDynamoDBAPI{}
	store := NewDynamoDBWithClient(api, "t", time.Hour)
	meta := map[string]string{
		"finding_arn": "arn:1",
		"severity":    "HIGH",
		attrDedupKey:  "must-not-overwrite",
		attrExpireAt:  "must-not-overwrite",
	}
	if _, err := store.TryReserve(context.Background(), "k", meta); err != nil {
		t.Fatalf("TryReserve: %v", err)
	}
	if v, ok := api.gotInput.Item["finding_arn"].(*types.AttributeValueMemberS); !ok || v.Value != "arn:1" {
		t.Fatalf("finding_arn = %v, want S 'arn:1'", api.gotInput.Item["finding_arn"])
	}
	if v, ok := api.gotInput.Item[attrDedupKey].(*types.AttributeValueMemberS); !ok || v.Value != "k" {
		t.Fatalf("dedup_key was overwritten by metadata: %v", api.gotInput.Item[attrDedupKey])
	}
}

func TestDynamoDB_Release_DeletesKey(t *testing.T) {
	api := &fakeDynamoDBAPI{}
	store := NewDynamoDBWithClient(api, "test-table", time.Hour)
	if err := store.Release(context.Background(), "key-1"); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if api.deleteCalls != 1 {
		t.Fatalf("DeleteItem calls = %d, want 1", api.deleteCalls)
	}
	if got := aws.ToString(api.gotDelete.TableName); got != "test-table" {
		t.Fatalf("table = %q, want test-table", got)
	}
	if len(api.gotDelete.Key) != 1 {
		t.Fatalf("key attributes = %v, want only %s", api.gotDelete.Key, attrDedupKey)
	}
	if v, ok := api.gotDelete.Key[attrDedupKey].(*types.AttributeValueMemberS); !ok || v.Value != "key-1" {
		t.Fatalf("dedup_key = %v, want S 'key-1'", api.gotDelete.Key[attrDedupKey])
	}
}

func TestDynamoDB_Release_Disabled_NoOp(t *testing.T) {
	api := &fakeDynamoDBAPI{}
	cases := []struct {
		name  string
		store *DynamoDBStore
	}{
		{name: "nil-receiver", store: nil},
		{name: "empty-table", store: NewDynamoDBWithClient(api, "", time.Hour)},
		{name: "nil-client", store: &DynamoDBStore{TableName: "t"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.store.Release(context.Background(), "key"); err != nil {
				t.Fatalf("Release: %v", err)
			}
		})
	}
	if api.deleteCalls != 0 {
		t.Fatalf("DeleteItem calls = %d, want 0 for a disabled store", api.deleteCalls)
	}
}

func TestDynamoDB_Release_ReturnsSDKError(t *testing.T) {
	sdkErr := errors.New("AccessDeniedException")
	api := &fakeDynamoDBAPI{deleteErr: sdkErr}
	store := NewDynamoDBWithClient(api, "t", time.Hour)
	err := store.Release(context.Background(), "key")
	if !errors.Is(err, sdkErr) {
		t.Fatalf("Release error = %v, want wrapped %v", err, sdkErr)
	}
}

// wrap returns a wrapped error using errors-package wrapping that errors.As
// can unwrap.
func wrap(msg string, inner error) error {
	return errWrapper{msg: msg, inner: inner}
}

type errWrapper struct {
	msg   string
	inner error
}

func (e errWrapper) Error() string { return e.msg + ": " + e.inner.Error() }
func (e errWrapper) Unwrap() error { return e.inner }

// stripFirstLine returns the input unchanged when it has one trailing newline;
// when the log emits a single record it's a single JSON line.
func stripFirstLine(b []byte) []byte {
	if i := bytes.IndexByte(b, '\n'); i > 0 {
		return b[:i]
	}
	return b
}

func stringPtr(s string) *string { return &s }
