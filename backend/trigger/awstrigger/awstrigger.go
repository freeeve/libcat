// Package awstrigger delivers trigger events over AWS messaging: an SQS
// queue (a worker consumes and rebuilds) or an EventBridge bus (fan-out to
// CodeBuild/Step Functions). Callers construct and own the clients.
package awstrigger

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"github.com/freeeve/libcatalog/backend/trigger"
)

// SQS sends each event as one message.
type SQS struct {
	Client   *sqs.Client
	QueueURL string
}

// Notify implements trigger.Notifier.
func (s SQS) Notify(ctx context.Context, e trigger.Event) error {
	body, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = s.Client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    &s.QueueURL,
		MessageBody: aws.String(string(body)),
	})
	return err
}

// EventBridge puts each event on a bus.
type EventBridge struct {
	Client *eventbridge.Client
	Bus    string
	Source string // e.g. "lcatd"
}

// Notify implements trigger.Notifier.
func (e EventBridge) Notify(ctx context.Context, ev trigger.Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	source := e.Source
	if source == "" {
		source = "lcatd"
	}
	_, err = e.Client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []ebtypes.PutEventsRequestEntry{{
			EventBusName: aws.String(e.Bus),
			Source:       aws.String(source),
			DetailType:   aws.String(ev.Kind),
			Detail:       aws.String(string(body)),
		}},
	})
	return err
}
