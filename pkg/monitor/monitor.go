package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// LogEvent represents a single log event from CloudWatch Logs.
type LogEvent struct {
	Timestamp time.Time
	RunnerID  string
	Message   string
}

// LogMonitor streams and retrieves CloudWatch Logs for a test run.
type LogMonitor struct {
	client   *cloudwatchlogs.Client
	logGroup string
	logger   *slog.Logger
}

// NewLogMonitor creates a new LogMonitor.
func NewLogMonitor(client *cloudwatchlogs.Client, logGroup string, logger *slog.Logger) *LogMonitor {
	return &LogMonitor{
		client:   client,
		logGroup: logGroup,
		logger:   logger,
	}
}

// StreamLogs streams log events in real-time, calling the handler for each event.
func (m *LogMonitor) StreamLogs(ctx context.Context, testName string, handler func(LogEvent)) error {
	var nextToken *string
	lastSeen := time.Now().Add(-1 * time.Minute).UnixMilli()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			input := &cloudwatchlogs.FilterLogEventsInput{
				LogGroupName: aws.String(m.logGroup),
				StartTime:    aws.Int64(lastSeen),
				Interleaved:  aws.Bool(true),
			}
			if nextToken != nil {
				input.NextToken = nextToken
			}

			res, err := m.client.FilterLogEvents(ctx, input)
			if err != nil {
				m.logger.Debug("failed to filter log events", "error", err)
				continue
			}

			for _, event := range res.Events {
				ts := time.UnixMilli(aws.ToInt64(event.Timestamp))
				handler(LogEvent{
					Timestamp: ts,
					RunnerID:  aws.ToString(event.LogStreamName),
					Message:   aws.ToString(event.Message),
				})
				if aws.ToInt64(event.Timestamp) > lastSeen {
					lastSeen = aws.ToInt64(event.Timestamp) + 1
				}
			}

			nextToken = res.NextToken
		}
	}
}

// GetAllLogs retrieves all log events for a test run.
func (m *LogMonitor) GetAllLogs(ctx context.Context, testName string) ([]LogEvent, error) {
	var events []LogEvent
	var nextToken *string

	for {
		input := &cloudwatchlogs.FilterLogEventsInput{
			LogGroupName: aws.String(m.logGroup),
			Interleaved:  aws.Bool(true),
		}
		if nextToken != nil {
			input.NextToken = nextToken
		}

		res, err := m.client.FilterLogEvents(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to filter log events: %w", err)
		}

		for _, event := range res.Events {
			events = append(events, LogEvent{
				Timestamp: time.UnixMilli(aws.ToInt64(event.Timestamp)),
				RunnerID:  aws.ToString(event.LogStreamName),
				Message:   aws.ToString(event.Message),
			})
		}

		nextToken = res.NextToken
		if nextToken == nil {
			break
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.Before(events[j].Timestamp)
	})

	return events, nil
}
