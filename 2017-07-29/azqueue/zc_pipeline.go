package azqueue

import (
	"github.com/Azure/azure-pipeline-go/pipeline"
	"context"
	"net/http"
	"net"
	"time"
	//"net/url"
	//"log"
)

// PipelineOptions is used to configure a request policy pipeline's retry policy and logging.
type PipelineOptions struct {
	// Log configures the pipeline's logging infrastructure indicating what information is logged and where.
	Log pipeline.LogOptions

	// Retry configures the built-in retry policy behavior.
	Retry RetryOptions

	// RequestLog configures the built-in request logging policy.
	RequestLog RequestLogOptions

	// Telemetry configures the built-in telemetry policy behavior.
	Telemetry TelemetryOptions
}

// NewPipeline creates a Pipeline using the specified credentials and options.
func NewPipeline(c Credential, o PipelineOptions) pipeline.Pipeline {
	if c == nil {
		panic("c can't be nil")
	}

	// Closest to API goes first; closest to the wire goes last
	f := []pipeline.Factory{
		NewTelemetryPolicyFactory(o.Telemetry),
		NewUniqueRequestIDPolicyFactory(),
		NewRetryPolicyFactory(o.Retry),
	}

	if _, ok := c.(*anonymousCredentialPolicyFactory); !ok {
		// For AnonymousCredential, we optimize out the policy factory since it doesn't do anything
		// NOTE: The credential's policy factory must appear close to the wire so it can sign any
		// changes made by other factories (like UniqueRequestIDPolicyFactory)
		f = append(f, c)
	}
	f = append(f,
		pipeline.MethodFactoryMarker(), // indicates at what stage in the pipeline the method factory is invoked
		NewRequestLogPolicyFactory(o.RequestLog))

	return pipeline.NewPipeline(f, pipeline.Options{HTTPSender: newDefaultHTTPClientFactory(), Log: o.Log})
}

func newDefaultHTTPClient() *http.Client {
	/*proxyURL, err := url.Parse("http://127.0.0.1:8888")
	if err != nil {
		log.Fatal(err)
	}*/

	// We want the Transport to have a large connection pool
	return &http.Client{
		Transport: &http.Transport{
			//Proxy: http.ProxyURL(proxyURL),
			// We use Dial instead of DialContext as DialContext has been reported to cause slower performance.
			Dial /*Context*/ : (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
				DualStack: true,
			}).Dial, /*Context*/
			MaxIdleConns:           0, // No limit
			MaxIdleConnsPerHost:    100,
			IdleConnTimeout:        90 * time.Second,
			TLSHandshakeTimeout:    10 * time.Second,
			ExpectContinueTimeout:  1 * time.Second,
			DisableKeepAlives:      false,
			DisableCompression:     false,
			MaxResponseHeaderBytes: 0,
			//ResponseHeaderTimeout:  time.Duration{},
			//ExpectContinueTimeout:  time.Duration{},
		},
	}
}


func newDefaultHTTPClientFactory() pipeline.Factory {
	return pipeline.FactoryFunc(func(next pipeline.Policy, po *pipeline.PolicyOptions) pipeline.PolicyFunc {
		return func(ctx context.Context, request pipeline.Request) (pipeline.Response, error) {
			r, err := newDefaultHTTPClient().Do(request.WithContext(ctx))
			if err != nil {
				err = pipeline.NewError(err, "HTTP request failed")
			}
			return pipeline.NewHTTPResponse(r), err
		}
	})
}