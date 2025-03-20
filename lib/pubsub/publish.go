package pubsub

import (
	"context"
	"fmt"
	"time"

	. "github.com/ForceCLI/force/lib"
	"github.com/pkg/errors"
)

func Publish(f *Force, channel string, message map[string]any) error {
	var err error

	Log.Info("Creating gRPC client...")
	client, err := NewGRPCClient(f)
	if err != nil {
		return errors.Wrap(err, "could not create gRPC client")
	}
	defer client.Close()

	Log.Info("Making GetTopic request...")
	topic, err := client.GetTopic(channel)
	if err == SessionExpiredError {
		err = f.RefreshSession()
		if err != nil {
			return errors.Wrap(err, "could not refresh session")
		}
		topic, err = client.GetTopic(channel)
	}
	if err != nil {
		client.Close()
		return errors.Wrap(err, "could not fetch topic")
	}

	if !topic.GetCanPublish() {
		client.Close()
		return fmt.Errorf("this user is not allowed to publish to the following topic: %s", channel)
	}

	err = client.Publish(channel, message)
	if err == SessionExpiredError {
		err = f.RefreshSession()
		if err != nil {
			return errors.Wrap(err, "could not refresh session")
		}
		err = client.Publish(channel, message)
	}
	if err != nil {
		Log.Info(fmt.Sprintf("error occurred while publishing to topic: %v", err))
	}
	return nil
}

func PublishMessages(f *Force, channel string, messages <-chan ForceRecord) error {
	return PublishMessagesWithContext(context.Background(), f, channel, messages)
}

func PublishMessagesWithContext(ctx context.Context, f *Force, channel string, messages <-chan ForceRecord) error {
	defer func() {
		for range messages {
			// drain messages
		}
	}()
	var err error

	Log.Info("Creating gRPC client...")
	client, err := NewGRPCClient(f)
	if err != nil {
		return errors.Wrap(err, "could not create gRPC client")
	}
	defer client.Close()

	Log.Info("Making GetTopic request...")
	topic, err := client.fetchTopic(channel)
	if err == SessionExpiredError {
		err = f.RefreshSession()
		if err != nil {
			return errors.Wrap(err, "could not refresh session")
		}
		topic, err = client.fetchTopic(channel)
	}
	if err != nil {
		client.Close()
		return errors.Wrap(err, "could not fetch topic")
	}

	if !topic.GetCanPublish() {
		client.Close()
		return fmt.Errorf("this user is not allowed to publish to the following topic: %s", channel)
	}
	waitForRecord := 5 * time.Second
	recordTimer := time.NewTimer(waitForRecord)
	defer recordTimer.Stop()

MESSAGES:
	for {
		recordTimer.Reset(waitForRecord)
		select {
		case <-ctx.Done():
			Log.Info("Context canceled. Not publishing more records.")
			break MESSAGES
		case message, ok := <-messages:
			if !ok {
				break MESSAGES
			}
			err = client.Publish(channel, message)
			if err == SessionExpiredError {
				err = f.RefreshSession()
				if err != nil {
					return errors.Wrap(err, "could not refresh session")
				}
				err = client.Publish(channel, message)
			}
			if err != nil {
				Log.Info(fmt.Sprintf("error occurred while publishing to topic: %v", err))
			}
		case <-recordTimer.C:
			Log.Info("Waiting for record to publish")
		}
	}
	return nil
}
