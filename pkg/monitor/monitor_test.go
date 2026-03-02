package monitor

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

// mockFilterLogEventsAPI is a mock implementation of FilterLogEventsAPI for testing.
type mockFilterLogEventsAPI struct {
	outputs []*cloudwatchlogs.FilterLogEventsOutput
	errors  []error
	callIdx int
}

func (m *mockFilterLogEventsAPI) FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error) {
	if m.callIdx >= len(m.outputs) {
		return nil, errors.New("no more mock responses configured")
	}

	output := m.outputs[m.callIdx]
	err := m.errors[m.callIdx]
	m.callIdx++

	return output, err
}

func TestNewLogMonitor(t *testing.T) {
	client := &mockFilterLogEventsAPI{}
	logGroup := "/k6-ec2/test"
	logger := slog.Default()

	monitor := NewLogMonitor(client, logGroup, logger)

	if monitor == nil {
		t.Fatal("NewLogMonitor() returned nil")
	}
	if monitor.client != client {
		t.Error("client not set correctly")
	}
	if monitor.logGroup != logGroup {
		t.Errorf("logGroup = %q, want %q", monitor.logGroup, logGroup)
	}
	if monitor.logger != logger {
		t.Error("logger not set correctly")
	}
}

func TestGetAllLogs_SinglePage(t *testing.T) {
	now := time.Now()
	events := []types.FilteredLogEvent{
		{
			Timestamp:     aws.Int64(now.UnixMilli()),
			LogStreamName: aws.String("runner-0"),
			Message:       aws.String("test message 1"),
		},
		{
			Timestamp:     aws.Int64(now.Add(1 * time.Second).UnixMilli()),
			LogStreamName: aws.String("runner-1"),
			Message:       aws.String("test message 2"),
		},
	}

	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events:    events,
				NextToken: nil,
			},
		},
		errors: []error{nil},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())
	logs, err := monitor.GetAllLogs(context.Background(), "test-run")

	if err != nil {
		t.Fatalf("GetAllLogs() error: %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("GetAllLogs() returned %d events, want 2", len(logs))
	}
	if logs[0].RunnerID != "runner-0" {
		t.Errorf("logs[0].RunnerID = %q, want runner-0", logs[0].RunnerID)
	}
	if logs[0].Message != "test message 1" {
		t.Errorf("logs[0].Message = %q, want 'test message 1'", logs[0].Message)
	}
	if logs[1].RunnerID != "runner-1" {
		t.Errorf("logs[1].RunnerID = %q, want runner-1", logs[1].RunnerID)
	}
	if logs[1].Message != "test message 2" {
		t.Errorf("logs[1].Message = %q, want 'test message 2'", logs[1].Message)
	}
}

func TestGetAllLogs_MultiplePages(t *testing.T) {
	now := time.Now()
	token1 := "token-1"

	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []types.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(now.UnixMilli()),
						LogStreamName: aws.String("runner-0"),
						Message:       aws.String("page 1 message 1"),
					},
					{
						Timestamp:     aws.Int64(now.Add(1 * time.Second).UnixMilli()),
						LogStreamName: aws.String("runner-1"),
						Message:       aws.String("page 1 message 2"),
					},
				},
				NextToken: &token1,
			},
			{
				Events: []types.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(now.Add(2 * time.Second).UnixMilli()),
						LogStreamName: aws.String("runner-2"),
						Message:       aws.String("page 2 message 1"),
					},
				},
				NextToken: nil,
			},
		},
		errors: []error{nil, nil},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())
	logs, err := monitor.GetAllLogs(context.Background(), "test-run")

	if err != nil {
		t.Fatalf("GetAllLogs() error: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("GetAllLogs() returned %d events, want 3", len(logs))
	}
	if logs[0].Message != "page 1 message 1" {
		t.Errorf("logs[0].Message = %q, want 'page 1 message 1'", logs[0].Message)
	}
	if logs[1].Message != "page 1 message 2" {
		t.Errorf("logs[1].Message = %q, want 'page 1 message 2'", logs[1].Message)
	}
	if logs[2].Message != "page 2 message 1" {
		t.Errorf("logs[2].Message = %q, want 'page 2 message 1'", logs[2].Message)
	}
}

func TestGetAllLogs_Error(t *testing.T) {
	expectedErr := errors.New("cloudwatch api error")

	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{nil},
		errors:  []error{expectedErr},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())
	logs, err := monitor.GetAllLogs(context.Background(), "test-run")

	if err == nil {
		t.Fatal("GetAllLogs() should return error when API fails")
	}
	if logs != nil {
		t.Errorf("GetAllLogs() returned logs on error, want nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("GetAllLogs() error = %v, want wrapped %v", err, expectedErr)
	}
}

func TestGetAllLogs_SortsEvents(t *testing.T) {
	now := time.Now()

	// Events returned out of order
	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []types.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(now.Add(3 * time.Second).UnixMilli()),
						LogStreamName: aws.String("runner-3"),
						Message:       aws.String("third"),
					},
					{
						Timestamp:     aws.Int64(now.UnixMilli()),
						LogStreamName: aws.String("runner-0"),
						Message:       aws.String("first"),
					},
					{
						Timestamp:     aws.Int64(now.Add(1 * time.Second).UnixMilli()),
						LogStreamName: aws.String("runner-1"),
						Message:       aws.String("second"),
					},
				},
				NextToken: nil,
			},
		},
		errors: []error{nil},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())
	logs, err := monitor.GetAllLogs(context.Background(), "test-run")

	if err != nil {
		t.Fatalf("GetAllLogs() error: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("GetAllLogs() returned %d events, want 3", len(logs))
	}

	// Verify sorted by timestamp
	if logs[0].Message != "first" {
		t.Errorf("logs[0].Message = %q, want 'first'", logs[0].Message)
	}
	if logs[1].Message != "second" {
		t.Errorf("logs[1].Message = %q, want 'second'", logs[1].Message)
	}
	if logs[2].Message != "third" {
		t.Errorf("logs[2].Message = %q, want 'third'", logs[2].Message)
	}

	// Verify timestamps are in ascending order
	for i := 1; i < len(logs); i++ {
		if !logs[i-1].Timestamp.Before(logs[i].Timestamp) {
			t.Errorf("logs[%d].Timestamp (%v) should be before logs[%d].Timestamp (%v)",
				i-1, logs[i-1].Timestamp, i, logs[i].Timestamp)
		}
	}
}

func TestGetAllLogs_EmptyResult(t *testing.T) {
	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events:    []types.FilteredLogEvent{},
				NextToken: nil,
			},
		},
		errors: []error{nil},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())
	logs, err := monitor.GetAllLogs(context.Background(), "test-run")

	if err != nil {
		t.Fatalf("GetAllLogs() error: %v", err)
	}
	if len(logs) != 0 {
		t.Errorf("GetAllLogs() returned %d events, want 0", len(logs))
	}
}

func TestStreamLogs_ContextCancellation(t *testing.T) {
	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{Events: []types.FilteredLogEvent{}, NextToken: nil},
		},
		errors: []error{nil},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	handlerCalled := false
	handler := func(LogEvent) {
		handlerCalled = true
	}

	err := monitor.StreamLogs(ctx, "test-run", handler)

	if err == nil {
		t.Fatal("StreamLogs() should return error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("StreamLogs() error = %v, want context.Canceled", err)
	}
	if handlerCalled {
		t.Error("handler should not be called when context is cancelled immediately")
	}
}

func TestStreamLogs_HandlerCalledForEvents(t *testing.T) {
	now := time.Now()

	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			{
				Events: []types.FilteredLogEvent{
					{
						Timestamp:     aws.Int64(now.UnixMilli()),
						LogStreamName: aws.String("runner-0"),
						Message:       aws.String("test message"),
					},
				},
				NextToken: nil,
			},
		},
		errors: []error{nil},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var receivedEvents []LogEvent
	handler := func(event LogEvent) {
		receivedEvents = append(receivedEvents, event)
		cancel() // Cancel after receiving first event
	}

	err := monitor.StreamLogs(ctx, "test-run", handler)

	// Should return context.Canceled after we cancel
	if err == nil {
		t.Fatal("StreamLogs() should return error")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("StreamLogs() error = %v, want context error", err)
	}
	if len(receivedEvents) != 1 {
		t.Errorf("handler called %d times, want 1", len(receivedEvents))
	}
	if len(receivedEvents) > 0 {
		if receivedEvents[0].RunnerID != "runner-0" {
			t.Errorf("event.RunnerID = %q, want runner-0", receivedEvents[0].RunnerID)
		}
		if receivedEvents[0].Message != "test message" {
			t.Errorf("event.Message = %q, want 'test message'", receivedEvents[0].Message)
		}
	}
}

func TestStreamLogs_ContinuesOnError(t *testing.T) {
	client := &mockFilterLogEventsAPI{
		outputs: []*cloudwatchlogs.FilterLogEventsOutput{
			nil, // First call fails
			nil, // Second call fails
			nil, // Third call fails
		},
		errors: []error{
			errors.New("temporary error"),
			errors.New("another error"),
			errors.New("yet another error"),
		},
	}

	monitor := NewLogMonitor(client, "/k6-ec2/test", slog.Default())

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()

	handlerCalled := false
	handler := func(LogEvent) {
		handlerCalled = true
	}

	// Run in goroutine and cancel after ticker fires at least twice (2s * 2 = 4s + margin)
	go func() {
		time.Sleep(4500 * time.Millisecond)
		cancel()
	}()

	err := monitor.StreamLogs(ctx, "test-run", handler)

	// Should eventually return context error, not API error
	if err == nil {
		t.Fatal("StreamLogs() should return error")
	}
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("StreamLogs() error = %v, want context error", err)
	}
	// Handler should not be called since all API calls failed
	if handlerCalled {
		t.Error("handler should not be called when API calls fail")
	}
	// Verify multiple API calls were attempted (callIdx should be >= 2)
	if client.callIdx < 2 {
		t.Errorf("StreamLogs() attempted %d API calls, want at least 2", client.callIdx)
	}
}
