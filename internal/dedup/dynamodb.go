// Package dedup implements idempotency-key persistence for parsers with
// at-least-once delivery semantics (Inspector2, future GuardDuty, etc.). The
// Store contract is a conditional PutItem with TTL: first sighting → true
// (caller owns the key, proceed with alerting), subsequent sightings → false
// (silenced).
//
// Implementation contract:
//   - ConditionalCheckFailedException detection uses errors.As, never string
//     matching.
//   - expire_at is persisted as a DynamoDB N (number) attribute so the TTL
//     scanner recognizes it.
package dedup

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	attrDedupKey       = "dedup_key"
	attrFirstAlertedAt = "first_alerted_at"
	attrExpireAt       = "expire_at"
)

// Deduplicator is the per-event idempotency seam injected into parsers.
type Deduplicator interface {
	// TryReserve atomically inserts a dedup key with the configured TTL.
	// Returns true when the caller owns the key (first sighting). Returns
	// false when the key already existed. SDK errors fall back to fail-open
	// (return true, nil) so a transient DynamoDB outage never silences a
	// security alert.
	TryReserve(ctx context.Context, key string, metadata map[string]string) (bool, error)
}

// PutItemAPI is the subset of the DynamoDB SDK the store depends on. Tests
// inject a fake; production wires the real *dynamodb.Client.
type PutItemAPI interface {
	PutItem(ctx context.Context, in *dynamodb.PutItemInput,
		optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

// Store is the legacy interface name kept for API parity with Step 1.
// Existing callers that depend on Store continue to compile.
type Store = Deduplicator

// DynamoDBStore is the production Deduplicator implementation backed by
// DynamoDB. When TableName is empty the store is disabled — TryReserve
// returns (true, nil) so the parser proceeds without dedup.
type DynamoDBStore struct {
	TableName string
	TTL       time.Duration
	client    PutItemAPI
	log       *slog.Logger
}

// NewDynamoDB returns a DynamoDBStore with the production SDK client. When
// tableName is empty the store is disabled — every TryReserve call returns
// (true, nil) without contacting DynamoDB.
func NewDynamoDB(cfg aws.Config, tableName string, ttl time.Duration) *DynamoDBStore {
	return &DynamoDBStore{
		TableName: tableName,
		TTL:       ttl,
		client:    dynamodb.NewFromConfig(cfg),
		log:       slog.Default(),
	}
}

// NewDynamoDBWithClient is the test seam — inject a PutItemAPI fake.
func NewDynamoDBWithClient(client PutItemAPI, tableName string, ttl time.Duration) *DynamoDBStore {
	return &DynamoDBStore{
		TableName: tableName,
		TTL:       ttl,
		client:    client,
		log:       slog.Default(),
	}
}

// WithLogger overrides the default slog.Default() logger.
func (s *DynamoDBStore) WithLogger(l *slog.Logger) *DynamoDBStore {
	s.log = l
	return s
}

// TryReserve performs the conditional PutItem.
//
// Hit (already seen) → (false, nil).
// Miss (first sighting) → (true, nil).
// SDK error → (true, nil) [fail-open]; the error is logged.
// Disabled (empty table) → (true, nil).
func (s *DynamoDBStore) TryReserve(ctx context.Context, key string, metadata map[string]string) (bool, error) {
	if s == nil || s.TableName == "" || s.client == nil {
		return true, nil
	}
	now := time.Now().UTC().Unix()
	expireAt := now + int64(s.TTL/time.Second)

	item := map[string]types.AttributeValue{
		attrDedupKey:       &types.AttributeValueMemberS{Value: key},
		attrFirstAlertedAt: &types.AttributeValueMemberN{Value: strconv.FormatInt(now, 10)},
		attrExpireAt:       &types.AttributeValueMemberN{Value: strconv.FormatInt(expireAt, 10)},
	}
	for k, v := range metadata {
		if _, exists := item[k]; exists {
			continue
		}
		item[k] = &types.AttributeValueMemberS{Value: v}
	}

	cond := "attribute_not_exists(" + attrDedupKey + ")"
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(s.TableName),
		Item:                item,
		ConditionExpression: aws.String(cond),
	})
	if err == nil {
		return true, nil
	}
	var conditional *types.ConditionalCheckFailedException
	if errors.As(err, &conditional) {
		return false, nil
	}
	s.logger().WarnContext(ctx, "dedup PutItem failed; failing open",
		"err", err,
		"key", key,
		"table", s.TableName,
	)
	return true, nil
}

// logger returns the configured slog logger, falling back to slog.Default() if
// the store was constructed via a zero value.
func (s *DynamoDBStore) logger() *slog.Logger {
	if s.log != nil {
		return s.log
	}
	return slog.Default()
}
