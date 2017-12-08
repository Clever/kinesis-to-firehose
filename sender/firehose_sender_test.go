package sender

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"

	kbc "github.com/Clever/amazon-kinesis-client-go/batchconsumer"
	"github.com/Clever/kinesis-to-firehose/sender/mock_firehoseiface"
)

func setupFirehoseSender(t *testing.T) *FirehoseSender {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()
	mockFirehoseAPI := mock_firehoseiface.NewMockFirehoseAPI(mockCtrl)
	return &FirehoseSender{
		streamName: "tester",
		client:     mockFirehoseAPI,
	}
}

func TestInitFirehoseWriter(t *testing.T) {
	_ = setupFirehoseSender(t)
}

func TestProcessMessageForES(t *testing.T) {
	sender := setupFirehoseSender(t)

	msg := `Apr  5 21:45:54 influx-service docker/0000aa112233[1234]: [httpd] 2017/04/05 ` +
		`21:45:54 172.17.42.1 - heka [05/Apr/2017:21:45:54 +0000] POST ` +
		`/write?db=foo&precision=ms HTTP/1.1 204 0 - Go 1.1 package http ` +
		`123456-1234-1234-b11b-000000000000 13.688672ms`
	_, tags, err := sender.ProcessMessage([]byte(msg))
	assert.NoError(t, err)
	assert.Contains(t, tags, sender.streamName)

	sender.isElasticsearch = true
	msg = `2017-08-16T04:37:52.901092+00:00 ip-10-0-102-159 production--haproxy-logs/` +
		`arn%3Aaws%3Aecs%3Aus-west-1%3A589690932525%3Atask%2F124cc8a5-0549-4149-922b-cd411b813d11` +
		`[3252]:  {"timestamp":1502858272,"http_status":200,"request_method":"POST","request":"/` +
		`.kibana-4/__kibanaQueryValidator/_validate/query?explain=true&ignore_unavailable=true",` +
		`"response_time":25,"termination_state":"----","request_body":"{"query":{"query_string":` +
		`{"query":"\"Franklin County School District\"","analyze_wildcard":true,` +
		`"lowercase_expanded_terms":false}}}","backend_name":"elasticsearch"}`
	_, _, err = sender.ProcessMessage([]byte(msg))
	assert.Error(t, err)
	assert.Equal(t, kbc.ErrMessageIgnored, err)

	sender.isElasticsearch = true
	msg = `2017-08-16T04:37:52.901092+00:00 ip-10-0-102-159 production--kinesis-cloudtrail-consumer/` +
		`arn%3Aaws%3Aecs%3Aus-west-1%3A589690932525%3Atask%2F124cc8a5-0549-4149-922b-cd411b813d11` +
		`[3252]: SEVERE: Received error line from subprocess [{"awsRegion":"us-east-1","deploy_env"` +
		`:"production","eventID":"93f997cc-e14e-4ca1-a5d1-9341c97da442","eventName":"GetBucketLocation"` +
		`,"eventSource":"s3.amazonaws.com","eventTime":"2017-12-06T19:18:22Z","eventType":"AwsApiCall"}]` +
		`for shard shardId-000000000000`
	_, _, err = sender.ProcessMessage([]byte(msg))
	assert.Error(t, err)
	assert.Equal(t, kbc.ErrMessageIgnored, err)

	sender.isElasticsearch = false
	msg = `2017-08-16T04:37:52.901092+00:00 ip-10-0-102-159 production--haproxy-logs/` +
		`arn%3Aaws%3Aecs%3Aus-west-1%3A589690932525%3Atask%2F124cc8a5-0549-4149-922b-cd411b813d11` +
		`[3252]:  {"timestamp":1502858272,"http_status":200,"request_method":"POST","request":"/` +
		`.kibana-4/__kibanaQueryValidator/_validate/query?explain=true&ignore_unavailable=true",` +
		`"response_time":25,"termination_state":"----","request_body":"{"query":{"query_string":` +
		`{"query":"\"Franklin County School District\"","analyze_wildcard":true,` +
		`"lowercase_expanded_terms":false}}}","backend_name":"elasticsearch"}`
	_, _, err = sender.ProcessMessage([]byte(msg))
	assert.NoError(t, err)
}

func TestMakeESSafe(t *testing.T) {
	sender := setupFirehoseSender(t)

	fields := map[string]interface{}{
		"_no_prefix_underscore": "yes",
		"no.dots.in.props":      "yes",
		"no-nesting":            map[string]interface{}{"nested": "nest"},
		"no-arrays":             []interface{}{"no", "array"},
		"":                      "empty",
	}
	expected := map[string]interface{}{
		"kv__no_prefix_underscore": "yes",
		"no_dots_in_props":         "yes",
		"no-nesting":               `{"nested":"nest"}`,
		"no-arrays":                `["no","array"]`,
		"":                         "empty",
	}

	assert.EqualValues(t, expected, sender.makeESSafe(fields))
}

func TestAddKVMetaFields(t *testing.T) {
	assert := assert.New(t)

	sender := setupFirehoseSender(t)
	fields := map[string]interface{}{
		"hi": "hello!",
		"_kvmeta": map[string]interface{}{
			"team":        "diversity",
			"kv_version":  "kv-routes",
			"kv_language": "understanding",
			"routes": []interface{}{
				map[string]interface{}{
					"type":       "metrics",
					"rule":       "all-combos",
					"series":     "1,1,2,6,24,120,720,5040",
					"dimensions": []interface{}{"fact", "orial"},
				},
				map[string]interface{}{
					"type":   "analytics",
					"rule":   "there's-app-invites-everywhere",
					"series": "there's-bts-in-the-air",
				},
				map[string]interface{}{
					"type":    "notifications",
					"rule":    "what's-the-catch",
					"channel": "slack-is-built-with-php",
					"message": "just like farmville",
				},
				map[string]interface{}{
					"type":        "alerts",
					"rule":        "last-call",
					"series":      "doing-it-til-we-fall",
					"dimensions":  []interface{}{"who", "where"},
					"stat_type":   "guage",
					"value_field": "status",
				},
			},
		},
	}
	fields = sender.addKVMetaFields(fields)

	assert.NotContains(fields, "_kvmeta")
	assert.Contains(fields, "kv_routes")
	assert.Contains(fields, "kv_team")
	assert.Contains(fields, "kv_language")
	assert.Contains(fields, "kv_version")

	assert.Equal(
		[]string{"all-combos", "there's-app-invites-everywhere", "what's-the-catch", "last-call"},
		fields["kv_routes"],
	)
	assert.Equal("diversity", fields["kv_team"])
	assert.Equal("understanding", fields["kv_language"])
	assert.Equal("kv-routes", fields["kv_version"])

}
