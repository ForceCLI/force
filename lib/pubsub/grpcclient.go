package pubsub

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	. "github.com/ForceCLI/force/lib"
	"github.com/pkg/errors"

	"github.com/ForceCLI/force/lib/pubsub/proto"
	"github.com/hamba/avro/v2"
	"github.com/linkedin/goavro/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	tokenHeader    = "accesstoken"
	instanceHeader = "instanceurl"
	tenantHeader   = "tenantid"

	GRPCEndpoint    = "api.pubsub.salesforce.com:7443"
	GRPCDialTimeout = 5 * time.Second
	GRPCCallTimeout = 5 * time.Second

	Appetite int32 = 5
)

var InvalidReplayIdError = errors.New("Invalid Replay Id")

type PubSubClient struct {
	session *ForceSession

	conn         *grpc.ClientConn
	pubSubClient proto.PubSubClient

	codecCache  map[string]*goavro.Codec
	schemaCache map[string]map[string]any
}

// Closes the underlying connection to the gRPC server
func (c *PubSubClient) Close() {
	c.conn.Close()
}

// Wrapper function around the GetTopic RPC. This will add the OAuth credentials and make a call to fetch data about a specific topic
func (c *PubSubClient) GetTopic(channel string) (*proto.TopicInfo, error) {
	var trailer metadata.MD

	req := &proto.TopicRequest{
		TopicName: channel,
	}

	ctx, cancelFn := context.WithTimeout(c.getAuthContext(), GRPCCallTimeout)
	defer cancelFn()

	resp, err := c.pubSubClient.GetTopic(ctx, req, grpc.Trailer(&trailer))
	logTrailer(trailer)

	if err != nil {
		if isAuthError(trailer.Get("error-code")) {
			return nil, SessionExpiredError
		}
		return nil, err
	}

	return resp, nil
}

// Wrapper function around the GetSchema RPC. This will add the OAuth credentials and make a call to fetch data about a specific schema
func (c *PubSubClient) GetSchema(schemaId string) (*proto.SchemaInfo, error) {
	var trailer metadata.MD

	req := &proto.SchemaRequest{
		SchemaId: schemaId,
	}

	ctx, cancelFn := context.WithTimeout(c.getAuthContext(), GRPCCallTimeout)
	defer cancelFn()

	resp, err := c.pubSubClient.GetSchema(ctx, req, grpc.Trailer(&trailer))
	logTrailer(trailer)

	if err != nil {
		if isAuthError(trailer.Get("error-code")) {
			return nil, SessionExpiredError
		}
		return nil, err
	}

	return resp, nil
}

// Wrapper function around the Subscribe RPC. This will add the OAuth credentials and create a separate streaming client that will be used to
// fetch data from the topic. This method will continuously consume messages unless an error occurs; if an error does occur then this method will
// return the last successfully consumed ReplayId as well as the error message. If no messages were successfully consumed then this method will return
// the same ReplayId that it originally received as a parameter
func (c *PubSubClient) Subscribe(channel string, replayPreset proto.ReplayPreset, replayId []byte, changesOnly bool) ([]byte, error) {
	ctx, cancelFn := context.WithCancel(c.getAuthContext())
	defer cancelFn()

	subscribeClient, err := c.pubSubClient.Subscribe(ctx)
	if err != nil {
		if isAuthError(subscribeClient.Trailer().Get("error-code")) {
			return replayId, SessionExpiredError
		}
		return replayId, err
	}
	defer subscribeClient.CloseSend()

	initialFetchRequest := &proto.FetchRequest{
		TopicName:    channel,
		ReplayPreset: replayPreset,
		NumRequested: Appetite,
	}
	if replayPreset == proto.ReplayPreset_CUSTOM && replayId != nil {
		initialFetchRequest.ReplayId = replayId
	}

	err = subscribeClient.Send(initialFetchRequest)
	// If the Send call returns an EOF error then print a log message but do not return immediately. Instead, let the Recv call (below) determine
	// if there's a more specific error that can be returned
	// See the SendMsg description at https://pkg.go.dev/google.golang.org/grpc#ClientStream
	if err == io.EOF {
		Log.Info("WARNING - EOF error returned from initial Send call, proceeding anyway")
	} else if err != nil {
		return replayId, err
	}

	requestedEvents := initialFetchRequest.NumRequested

	curReplayId := replayId
	for {
		Log.Info("Waiting for events...")
		resp, err := subscribeClient.Recv()
		if err == io.EOF {
			logTrailer(subscribeClient.Trailer())
			return curReplayId, fmt.Errorf("stream closed")
		} else if err != nil {
			if isAuthError(subscribeClient.Trailer().Get("error-code")) {
				return curReplayId, SessionExpiredError
			}
			if isInvalidReplayIdError(subscribeClient.Trailer().Get("error-code")) {
				return curReplayId, InvalidReplayIdError
			}
			logTrailer(subscribeClient.Trailer())
			return curReplayId, err
		}

		for _, event := range resp.Events {
			codec, err := c.fetchCodec(event.GetEvent().GetSchemaId())
			if err != nil {
				return curReplayId, err
			}

			parsed, _, err := codec.NativeFromBinary(event.GetEvent().GetPayload())
			if err != nil {
				return curReplayId, err
			}

			body, ok := parsed.(map[string]any)
			if !ok {
				return curReplayId, fmt.Errorf("error casting parsed event: %v", body)
			}

			// Again, this should be stored in a persistent external datastore instead of a variable
			curReplayId = event.GetReplayId()

			if changesOnly {
				// If this is a Change Data Capture event, there will be a ChangeEventHeader object that contains
				// changedFields, diffFields, and nulledFields.  We can parse these
				// bitmap fields so that we can display only the changed fields.
				schema, err := c.fetchSchema(event.GetEvent().GetSchemaId())
				if err != nil {
					return curReplayId, err
				}
				body = parseBody(body, schema)
			}

			j, err := json.Marshal(body)
			if err != nil {
				return curReplayId, err
			}
			fmt.Println(string(j))
			Log.Info(fmt.Sprintf("ReplayId (%s): %s", channel, base64.StdEncoding.EncodeToString(curReplayId)))

			// decrement our counter to keep track of how many events have been requested but not yet processed. If we're below our configured
			// batch size then proactively request more events to stay ahead of the processor
			requestedEvents--
			if requestedEvents < Appetite {
				Log.Info("Sending next FetchRequest...")
				fetchRequest := &proto.FetchRequest{
					TopicName:    channel,
					NumRequested: Appetite,
				}

				err = subscribeClient.Send(fetchRequest)
				// If the Send call returns an EOF error then print a log message but do not return immediately. Instead, let the Recv call (above) determine
				// if there's a more specific error that can be returned
				// See the SendMsg description at https://pkg.go.dev/google.golang.org/grpc#ClientStream
				if err == io.EOF {
					Log.Info("WARNING - EOF error returned from subsequent Send call, proceeding anyway")
				} else if err != nil {
					return curReplayId, err
				}

				requestedEvents += fetchRequest.NumRequested
			}
		}
	}
}

func (c *PubSubClient) fetchSchema(schemaId string) (map[string]any, error) {
	var schemaJson map[string]any
	schemaJson, ok := c.schemaCache[schemaId]
	if ok {
		return schemaJson, nil
	}

	Log.Info("Making GetSchema request for uncached schema...")
	schema, err := c.GetSchema(schemaId)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(schema.GetSchemaJson()), &schemaJson)
	if err != nil {
		return nil, errors.Wrap(err, "could not unmarshal schema to json")
	}

	c.schemaCache[schemaId] = schemaJson

	return schemaJson, nil
}

// Unexported helper function to retrieve the cached codec from the PubSubClient's schema cache. If the schema ID is not found in the cache
// then a GetSchema call is made and the corresponding codec is cached for future use
func (c *PubSubClient) fetchCodec(schemaId string) (*goavro.Codec, error) {
	codec, ok := c.codecCache[schemaId]
	if ok {
		return codec, nil
	}

	schema, err := c.fetchSchema(schemaId)
	if err != nil {
		return nil, err
	}

	Log.Info("Creating codec from schema...")
	schemaJson, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	codec, err = goavro.NewCodec(string(schemaJson))
	if err != nil {
		return nil, err
	}

	c.codecCache[schemaId] = codec

	return codec, nil
}

// Wrapper function around the Publish RPC. This will add the OAuth credentials and produce a single hardcoded event to the specified topic.
func (c *PubSubClient) Publish(channel string, message map[string]any) error {
	topic, err := c.GetTopic(channel)
	if err != nil {
		return err
	}
	schema, err := c.GetSchema(topic.GetSchemaId())
	if err != nil {
		return err
	}
	Log.Info("Creating codec from schema...")
	codec, err := avro.Parse(schema.SchemaJson)
	if err != nil {
		return err
	}

	if message["CreatedDate"] == nil {
		message["CreatedDate"] = time.Now().Unix()
	}
	if message["CreatedById"] == nil {
		message["CreatedById"] = c.session.UserInfo.UserId
	}
	switch message["CreatedDate"].(type) {
	case int:
		message["CreatedDate"] = int64(message["CreatedDate"].(int))
	case time.Time:
		message["CreatedDate"] = message["CreatedDate"].(time.Time).Unix()
	}

	payload, err := avro.Marshal(codec, message)
	if err != nil {
		return err
	}

	var trailer metadata.MD

	req := &proto.PublishRequest{
		TopicName: channel,
		Events: []*proto.ProducerEvent{
			{
				SchemaId: schema.GetSchemaId(),
				Payload:  payload,
			},
		},
	}

	ctx, cancelFn := context.WithTimeout(c.getAuthContext(), GRPCCallTimeout)
	defer cancelFn()

	pubResp, err := c.pubSubClient.Publish(ctx, req, grpc.Trailer(&trailer))
	logTrailer(trailer)

	if err != nil {
		var errors []string
		for _, r := range pubResp.GetResults() {
			errors = append(errors, r.GetError().GetMsg())
			Log.Info("Got Error", r.GetError().GetMsg())
		}
		if isAuthError(errors) {
			return SessionExpiredError
		}
		return err
	}

	result := pubResp.GetResults()
	if result == nil {
		return fmt.Errorf("nil result returned when publishing to %s", channel)
	}

	if err := result[0].GetError(); err != nil {
		return fmt.Errorf(result[0].GetError().GetMsg())
	}

	return nil
}

// Returns a new context with the necessary authentication parameters for the gRPC server
func (c *PubSubClient) getAuthContext() context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs(
		tokenHeader, c.session.AccessToken,
		instanceHeader, c.session.InstanceUrl,
		tenantHeader, c.session.UserInfo.OrgId,
	))
}

// Creates a new connection to the gRPC server and returns the wrapper struct
func NewGRPCClient(f *Force) (*PubSubClient, error) {
	dialOpts := []grpc.DialOption{
		grpc.WithBlock(),
	}

	if GRPCEndpoint == "localhost:7011" {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		certs := getCerts()
		creds := credentials.NewClientTLSFromCert(certs, "")
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(creds))
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), GRPCDialTimeout)
	defer cancelFn()

	conn, err := grpc.DialContext(ctx, GRPCEndpoint, dialOpts...)
	if err != nil {
		return nil, err
	}

	return &PubSubClient{
		conn:         conn,
		session:      f.Credentials,
		pubSubClient: proto.NewPubSubClient(conn),
		codecCache:   make(map[string]*goavro.Codec),
		schemaCache:  make(map[string]map[string]any),
	}, nil
}

func isAuthError(values []string) bool {
	for _, v := range values {
		if v == "sfdc.platform.eventbus.grpc.service.auth.error" {
			return true
		}
	}
	return false
}

func isInvalidReplayIdError(values []string) bool {
	for _, v := range values {
		if v == "sfdc.platform.eventbus.grpc.subscription.fetch.replayid.corrupted" {
			return true
		}
	}
	return false
}

// Fetches system certs and returns them if possible. If unable to fetch system certs then an empty cert pool is returned instead
func getCerts() *x509.CertPool {
	if certs, err := x509.SystemCertPool(); err == nil {
		return certs
	}

	return x509.NewCertPool()
}

// Helper function to display trailers on the console in a more readable format
func logTrailer(trailer metadata.MD) {
	if len(trailer) == 0 {
		Log.Info("no trailers returned")
		return
	}

	Log.Info("beginning of trailers")
	for key, val := range trailer {
		Log.Info(fmt.Sprintf("[trailer] = %s, [value] = %s", key, val))
	}
	Log.Info("end of trailers")
}
