package lib

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// ReplayDebugLevelName is the DeveloperName of the DebugLevel used while
// capturing a log for the replay debugger.
const ReplayDebugLevelName = "Force_CLI_Replay"

// replayTraceDuration is how long a trace flag set for the replay debugger
// stays active if it is not restored, such as when force is interrupted.
const replayTraceDuration = 30 * time.Minute

const traceFlagTimeFormat = "2006-01-02T15:04:05.000+0000"

// ReplayDebugLevel returns the ID of the Force_CLI_Replay DebugLevel,
// creating it if it does not exist. Apex code is logged at FINEST, which
// records each statement and variable assignment; other categories are
// logged only as far as the debugger uses them, to keep the log small.
func (f *Force) ReplayDebugLevel() (string, error) {
	result, err := f.Query(fmt.Sprintf("SELECT Id FROM DebugLevel WHERE DeveloperName = '%s'", ReplayDebugLevelName), func(o *QueryOptions) { o.IsTooling = true })
	if err != nil {
		return "", errors.Wrap(err, "querying debug level")
	}
	if len(result.Records) > 0 {
		id, _ := result.Records[0]["Id"].(string)
		return id, nil
	}
	created, err := f.CreateToolingRecord("DebugLevel", map[string]string{
		"DeveloperName": ReplayDebugLevelName,
		"MasterLabel":   ReplayDebugLevelName,
		"ApexCode":      "Finest",
		"ApexProfiling": "Info",
		"Visualforce":   "Finer",
		"Callout":       "None",
		"Database":      "None",
		"System":        "None",
		"Validation":    "None",
		"Workflow":      "None",
	})
	if err != nil {
		return "", errors.Wrap(err, "creating debug level")
	}
	if !created.Success || created.Id == "" {
		return "", fmt.Errorf("creating debug level %s failed", ReplayDebugLevelName)
	}
	return created.Id, nil
}

// EnableReplayTraceFlag sets the current user's DEVELOPER_LOG trace flag to
// the Force_CLI_Replay debug level for 30 minutes, so that logs of code run
// by the user can be replayed. A user has at most one DEVELOPER_LOG trace
// flag, so an existing one is changed rather than a second one created.
//
// The returned function puts the trace flag back: an existing trace flag
// gets its debug level, start date, and expiration date back, and a trace
// flag created here is deleted.
func (f *Force) EnableReplayTraceFlag() (restore func() error, err error) {
	if err = f.EnsureUserInfo(); err != nil {
		return nil, err
	}
	debugLevelId, err := f.ReplayDebugLevel()
	if err != nil {
		return nil, err
	}
	userId := f.Credentials.UserInfo.UserId
	result, err := f.Query(fmt.Sprintf("SELECT Id, DebugLevelId, StartDate, ExpirationDate FROM TraceFlag WHERE TracedEntityId = '%s' AND LogType = 'DEVELOPER_LOG'", userId), func(o *QueryOptions) { o.IsTooling = true })
	if err != nil {
		return nil, errors.Wrap(err, "querying trace flags")
	}
	now := time.Now().UTC()
	expiration := now.Add(replayTraceDuration).Format(traceFlagTimeFormat)

	if len(result.Records) == 0 {
		created, err := f.CreateToolingRecord("TraceFlag", map[string]string{
			"TracedEntityId": userId,
			"LogType":        "DEVELOPER_LOG",
			"DebugLevelId":   debugLevelId,
			"StartDate":      now.Format(traceFlagTimeFormat),
			"ExpirationDate": expiration,
		})
		if err != nil {
			return nil, errors.Wrap(err, "creating trace flag")
		}
		if !created.Success || created.Id == "" {
			return nil, errors.New("creating trace flag failed")
		}
		return func() error {
			return errors.Wrapf(f.DeleteToolingRecord("TraceFlag", created.Id), "deleting trace flag %s", created.Id)
		}, nil
	}

	record := result.Records[0]
	id, _ := record["Id"].(string)
	original := map[string]string{}
	original["DebugLevelId"], _ = record["DebugLevelId"].(string)
	original["ExpirationDate"], _ = record["ExpirationDate"].(string)
	changes := map[string]string{
		"DebugLevelId":   debugLevelId,
		"ExpirationDate": expiration,
	}
	if start, ok := record["StartDate"].(string); ok && start != "" {
		original["StartDate"] = start
		changes["StartDate"] = now.Format(traceFlagTimeFormat)
	}
	if err = f.UpdateToolingRecord("TraceFlag", id, changes); err != nil {
		return nil, errors.Wrapf(err, "updating trace flag %s", id)
	}
	return func() error {
		err := f.UpdateToolingRecord("TraceFlag", id, original)
		if err != nil && expiredBefore(original["ExpirationDate"], time.Now()) {
			// Salesforce does not accept an expiration date in the past,
			// so a trace flag that had expired is expired again now.
			original["ExpirationDate"] = time.Now().UTC().Add(time.Minute).Format(traceFlagTimeFormat)
			err = f.UpdateToolingRecord("TraceFlag", id, original)
		}
		return errors.Wrapf(err, "restoring trace flag %s", id)
	}, nil
}

func expiredBefore(date string, t time.Time) bool {
	parsed, err := time.Parse(traceFlagTimeFormat, date)
	return err == nil && parsed.Before(t)
}
