package pubsub

import (
	"fmt"
	"strconv"

	"encoding/base64"
	"encoding/binary"

	. "github.com/ForceCLI/force/lib"
	"github.com/pkg/errors"

	"github.com/ForceCLI/force/lib/pubsub/proto"
)

func Subscribe(f *Force, channel string, replayId string, replayPreset proto.ReplayPreset, parseChanges bool) error {
	var curReplayId []byte
	var err error
	if replayId != "" {
		curReplayId, err = parseReplayId(replayId)
		if err != nil {
			return fmt.Errorf("Could not parse replay id: %w", err)
		}
	}

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

	if !topic.GetCanSubscribe() {
		client.Close()
		return fmt.Errorf("this user is not allowed to subscribe to the following topic: %s", channel)
	}

	for {
		Log.Info("Subscribing to topic...")

		// use the user-provided ReplayPreset by default, but if the curReplayId variable has a non-nil value then assume that we want to
		// consume from a custom offset. The curReplayId will have a non-nil value if the user explicitly set the ReplayId or if a previous
		// subscription attempt successfully processed at least one event before crashing
		if curReplayId != nil {
			replayPreset = proto.ReplayPreset_CUSTOM
		}

		// In the happy path the Subscribe method should never return, it will just process events indefinitely. In the unhappy path
		// (i.e., an error occurred) the Subscribe method will return both the most recently processed ReplayId as well as the error message.
		// The error message will be logged for the user to see and then we will attempt to re-subscribe with the ReplayId on the next iteration
		// of this for loop
		curReplayId, err = client.Subscribe(channel, replayPreset, curReplayId, parseChanges)
		if err == SessionExpiredError {
			err = f.RefreshSession()
			if err != nil {
				return errors.Wrap(err, "could not refresh session")
			}
		}
		if err == InvalidReplayIdError {
			return errors.Wrap(err, fmt.Sprintf("could not subscribe starting at replay id: %s", base64.StdEncoding.EncodeToString(curReplayId)))
		}
		if err != nil {
			Log.Info(fmt.Sprintf("error occurred while subscribing to topic: %v", err))
		}
	}
}

// Try to parse replay id as a number first, then as a base64-encoded byte
// array
func parseReplayId(replayId string) ([]byte, error) {
	buf := make([]byte, 8)
	if n, err := strconv.ParseInt(replayId, 10, 64); err == nil {
		binary.BigEndian.PutUint64(buf, uint64(n))
		return buf, nil
	}
	buf, err := base64.StdEncoding.DecodeString(replayId)
	if err != nil {
		return buf, errors.Wrap(err, "could not decode replay id")
	}
	return buf, nil
}
