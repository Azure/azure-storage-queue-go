package azqueue_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/Azure/azure-pipeline-go/pipeline"
	"github.com/Azure/azure-storage-queue-go/azqueue"
)

// https://godoc.org/github.com/fluhus/godoc-tricks

// Please set the ACCOUNT_NAME and ACCOUNT_KEY environment variables to your storage account's
// name and account key, before running the examples.
func accountInfo() (string, string) {
	return os.Getenv("ACCOUNT_NAME"), os.Getenv("ACCOUNT_KEY")
}

// This example shows how to get started using the Azure Storage Queue SDK for Go.
func Example() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is used to access your account.
	credential, err := azqueue.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}

	// Create a request pipeline that is used to process HTTP(S) requests and responses. It requires
	// your account credentials. In more advanced scenarios, you can configure telemetry, retry policies,
	// logging, and other options. Also, you can configure multiple request pipelines for different scenarios.
	p := azqueue.NewPipeline(credential, azqueue.PipelineOptions{})

	// From the Azure portal, get your Storage account queue service URL endpoint.
	// The URL typically looks like this:
	u, _ := url.Parse(fmt.Sprintf("https://%s.queue.core.windows.net", accountName))

	// Create an ServiceURL object that wraps the service URL and a request pipeline.
	serviceURL := azqueue.NewServiceURL(*u, p)

	// Now, you can use the serviceURL to perform various queue operations.

	// All HTTP operations allow you to specify a Go context.Context object to control cancellation/timeout.
	ctx := context.TODO() // This example uses a never-expiring context.

	// Create a URL that references a queue in your Azure Storage account.
	// This returns a QueueURL object that wraps the queue's URL and a request pipeline (inherited from serviceURL)
	queueURL := serviceURL.NewQueueURL("examplequeue") // Queue names require lowercase

	// The code below shows how to create the queue. It is common to create a queue and never delete it:
	_, err = queueURL.Create(ctx, azqueue.Metadata{})
	if err != nil {
		log.Fatal(err)
	}

	// The code below shows how a client application enqueues 2 messages into the queue:
	// Create a URL allowing you to manipulate a queue's messages.
	// This returns a MessagesURL object that wraps the queue's messages URL and a request pipeline (inherited from queueURL)
	messagesURL := queueURL.NewMessagesURL()

	// Enqueue 2 messages
	_, err = messagesURL.Enqueue(ctx, "This is message 1", time.Second*0, time.Minute)
	if err != nil {
		log.Fatal(err)
	}
	_, err = messagesURL.Enqueue(ctx, "This is message 2", time.Second*0, time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	// The code below shows how a client or server can determine the approximate count of messages in the queue:
	props, err := queueURL.GetProperties(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Approximate number of messages in the queue=%d\n", props.ApproximateMessagesCount())

	// The code below shows how to initialize a service that wishes to process messages:
	const concurrentMsgProcessing = 16 // Set this as you desire
	msgCh := make(chan *azqueue.DequeuedMessage, concurrentMsgProcessing)
	const poisonMessageDequeueThreshold = 4 // Indicates how many times a message is attempted to be processed before considering it a poison message

	// Create goroutines that can process messages in parallel
	for n := 0; n < concurrentMsgProcessing; n++ {
		go func(msgCh <-chan *azqueue.DequeuedMessage) {
			for {
				msg := <-msgCh // Get a message from the channel

				// Create a URL allowing you to manipulate this message.
				// This returns a MessageIDURL object that wraps the this message's URL and a request pipeline (inherited from messagesURL)
				msgIDURL := messagesURL.NewMessageIDURL(msg.ID)
				popReceipt := msg.PopReceipt // This message's most-recent pop receipt

				if msg.DequeueCount > poisonMessageDequeueThreshold {
					// This message has attempted to be processed too many times; treat it as a poison message
					// DO NOT attempt to process this message
					// Log this message as a poison message somewhere (code not shown)
					// Delete this poison message from the queue so it will never be dequeued again
					msgIDURL.Delete(ctx, popReceipt)
					continue // Process a different message
				}

				// This message is not a poison message, process it (this example just displays it):
				fmt.Print(msg.Text + "\n")

				// NOTE: You can examine/use any of the message's other properties as you desire:
				_, _, _ = msg.InsertionTime, msg.ExpirationTime, msg.NextVisibleTime

				// OPTIONAL: while processing a message, you can update the message's visibility timeout
				// (to prevent other servers from dequeuing the same message simultaneously) and update the
				// message's text (to prevent some successfully-completed processing from re-executing the
				// next time this message is dequeued):
				update, err := msgIDURL.Update(ctx, popReceipt, time.Second*20, "updated msg")
				if err != nil {
					log.Fatal(err)
				}
				popReceipt = update.PopReceipt // Performing any operation on a message ID always requires the most recent pop receipt

				// After processing the message, delete it from the queue so it won't be dequeued ever again:
				_, err = msgIDURL.Delete(ctx, popReceipt)
				if err != nil {
					log.Fatal(err)
				}
				// Loop around to process the next message
			}
		}(msgCh)
	}

	// The code below shows the service's infinite loop that dequeues messages and dispatches them in batches for processsing:
	for {
		// Try to dequeue a batch of messages from the queue
		dequeue, err := messagesURL.Dequeue(ctx, azqueue.QueueMaxMessagesDequeue, 10*time.Second)
		if err != nil {
			log.Fatal(err)
		}
		if dequeue.NumMessages() == 0 {
			// The queue was empty; sleep a bit and try again
			// Shorter time means higher costs & less latency to dequeue a message
			// Higher time means lower costs & more latency to dequeue a message
			time.Sleep(time.Second * 10)
		} else {
			// We got some messages, put them in the channel so that many can be processed in parallel:
			// NOTE: The queue does not guarantee FIFO ordering & processing messages in parallel also does
			// not preserve FIFO ordering. So, the "Output:" order below is not guaranteed but usually works.
			for m := int32(0); m < dequeue.NumMessages(); m++ {
				msgCh <- dequeue.Message(m)
			}
		}
		// This batch of dequeued messages are in the channel, dequeue another batch
		break // NOTE: For this example only, break out of the infinite loop so this example terminates
	}

	time.Sleep(time.Second * 10) // For this example, delay in hopes that both messages complete processing before the example terminates

	// This example deletes the queue (to clean up) but normally, you never delete a queue.
	_, err = queueURL.Delete(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Unordered output:
	// Approximate number of messages in the queue=2
	// This is message 1
	// This is message 2
}

// This example shows how you can configure a pipeline for making HTTP requests to the Azure Storage Queue Service.
func ExampleNewPipeline() {
	// This example shows how to wire in your own logging mechanism (this example uses
	// Go's standard logger to write log information to standard error)
	logger := log.New(os.Stderr, "", log.Ldate|log.Lmicroseconds)

	// Create/configure a request pipeline options object.
	// All PipelineOptions' fields are optional; reasonable defaults are set for anything you do not specify
	po := azqueue.PipelineOptions{
		// Set RetryOptions to control how HTTP request are retried when retryable failures occur
		Retry: azqueue.RetryOptions{
			Policy:        azqueue.RetryPolicyExponential, // Use exponential backoff as opposed to fixed
			MaxTries:      3,                              // Try at most 3 times to perform the operation (set to 1 to disable retries)
			TryTimeout:    time.Second * 3,                // Maximum time allowed for any single try
			RetryDelay:    time.Second * 1,                // Backoff amount for each retry (exponential or fixed)
			MaxRetryDelay: time.Second * 3,                // Max delay between retries
		},

		// Set RequestLogOptions to control how each HTTP request & its response is logged
		RequestLog: azqueue.RequestLogOptions{
			LogWarningIfTryOverThreshold: time.Millisecond * 200, // A successful response taking more than this time to arrive is logged as a warning
		},

		// Set LogOptions to control what & where all pipeline log events go
		Log: pipeline.LogOptions{
			ShouldLog: func(level pipeline.LogLevel) bool {
				return level <= pipeline.LogWarning // Log all events from warning to more severe
			},
			Log: func(s pipeline.LogLevel, m string) { // This func is called to log each event
				// This method is not called for filtered-out severities.
				logger.Output(2, m) // This example uses Go's standard logger
			}},
	}

	// Create a request pipeline object configured with credentials and with pipeline options. Once created,
	// a pipeline object is goroutine-safe and can be safely used with many XxxURL objects simultaneously.
	p := azqueue.NewPipeline(azqueue.NewAnonymousCredential(), po) // A pipeline always requires some credential object

	// Once you've created a pipeline object, associate it with an XxxURL object so that you can perform HTTP requests with it.
	u, _ := url.Parse("https://myaccount.queue.core.windows.net")
	serviceURL := azqueue.NewServiceURL(*u, p)
	// Use the serviceURL as desired...

	// NOTE: When you use an XxxURL object to create another XxxURL object, the new XxxURL object inherits the
	// same pipeline object as its parent. For example, the queueURL and messagesURL objects (created below)
	// all share the same pipeline. Any HTTP operations you perform with these objects share the behavior (retry, logging, etc.)
	queueURL := serviceURL.NewQueueURL("myqueue1")
	messagesURL := queueURL.NewMessagesURL()

	// If you'd like to perform some operations with different behavior, create a new pipeline object and
	// associate it with a new XxxURL object by passing the new pipeline to the XxxURL object's WithPipeline method.

	// In this example, I reconfigure the retry policies, create a new pipeline, and then create a new
	// QueueURL object that has the same URL as its parent.
	po.Retry = azqueue.RetryOptions{
		Policy:        azqueue.RetryPolicyFixed, // Use fixed backoff as opposed to exponential
		MaxTries:      4,                        // Try at most 4 times to perform the operation (set to 1 to disable retries)
		TryTimeout:    time.Minute * 1,          // Maximum time allowed for any single try
		RetryDelay:    time.Second * 5,          // Backoff amount for each retry (exponential or fixed)
		MaxRetryDelay: time.Second * 10,         // Max delay between retries
	}
	newQueueURL := queueURL.WithPipeline(azqueue.NewPipeline(azqueue.NewAnonymousCredential(), po))

	// Now, any MessagesURL object created using newQueueURL inherits the pipeline with the new retry policy.
	newMessagesURL := newQueueURL.NewMessagesURL()
	_, _ = messagesURL, newMessagesURL // Avoid compiler's "declared and not used" error
}

func ExampleStorageError() {
	// This example shows how to handle errors returned from various XxxURL methods. All these methods return an
	// object implementing the pipeline.Response interface and an object implementing Go's error interface.
	// The error result is nil if the request was successful; your code can safely use the Response interface object.
	// If error is non-nil, the error could be due to:

	// 1. An invalid argument passed to the method. You should not write code to handle these errors;
	//    instead, fix these errors as they appear during development/testing.

	// 2. A network request didn't reach an Azure Storage Service. This usually happens due to a bad URL or
	//    faulty networking infrastructure (like a router issue). In this case, an object implementing the
	//    net.Error interface will be returned. The net.Error interface offers Timeout and Temporary methods
	//    which return true if the network error is determined to be a timeout or temporary condition. If
	//    your pipeline uses the retry policy factory, then this policy looks for Timeout/Temporary and
	//    automatically retries based on the retry options you've configured. Because of the retry policy,
	//    your code will usually not call the Timeout/Temporary methods explicitly other than possibly logging
	//    the network failure.

	// 3. A network request did reach the Azure Storage Service but the service failed to perform the
	//    requested operation. In this case, an object implementing the StorageError interface is returned.
	//    The StorageError interface also implements the net.Error interface and, if you use the retry policy,
	//    you would most likely ignore the Timeout/Temporary methods. However, the StorageError interface exposes
	//    richer information such as a service error code, an error description, details data, and the
	//    service-returned http.Response. And, from the http.Response, you can get the initiating http.Request.

	u, _ := url.Parse("https://myaccount.queue.core.windows.net/queue2") // Assumes the 'myaccount' storage account exists
	queueURL := azqueue.NewQueueURL(*u, azqueue.NewPipeline(azqueue.NewAnonymousCredential(), azqueue.PipelineOptions{}))
	create, err := queueURL.Create(context.Background(), azqueue.Metadata{})

	if err != nil { // An error occurred
		if serr, ok := err.(azqueue.StorageError); ok { // This error is a Service-specific error
			// StorageError also implements net.Error so you could call its Timeout/Temporary methods if you want.
			switch serr.ServiceCode() { // Compare serviceCode to various ServiceCodeXxx constants
			case azqueue.ServiceCodeQueueAlreadyExists:
				// You can also look at the http.Response object that failed.
				if failedResponse := serr.Response(); failedResponse != nil {
					// From the response object, you can get the initiating http.Request object
					failedRequest := failedResponse.Request
					_ = failedRequest // Avoid compiler's "declared and not used" error
				}

			case azqueue.ServiceCodeQueueBeingDeleted:
				// Handle this error ...
			default:
				// Handle other errors ...
			}
		}
		log.Fatal(err) // Error is not due to Azure Storage service; networking infrastructure failure
	}

	// If err is nil, then the method was successful; use the response to access the result
	_ = create // Avoid compiler's "declared and not used" error
}

// This example shows how to break a URL into its parts so you can
// examine and/or change some of its values and then construct a new URL.
func ExampleQueueURLParts() {
	// Let's start with a URL that identifies a queue that also contains a Shared Access Signature (SAS):
	u, _ := url.Parse("https://myaccount.queue.core.windows.net/aqueue/messages/30dd879c-ee2f-11db-8314-0800200c9a66?" +
		"sv=2015-02-21&sr=q&st=2111-01-09T01:42:34.936Z&se=2222-03-09T01:42:34.936Z&sp=rup&sip=168.1.5.60-168.1.5.70&" +
		"spr=https,http&si=myIdentifier&ss=q&srt=o&sig=92836758923659283652983562==")

	// You can parse this URL into its constituent parts:
	parts := azqueue.NewQueueURLParts(*u)

	// Now, we access the parts (this example prints them).
	fmt.Println(parts.Host, parts.QueueName)
	sas := parts.SAS
	fmt.Println(sas.Version(), sas.Resource(), sas.StartTime(), sas.ExpiryTime(), sas.Permissions(),
		sas.IPRange(), sas.Protocol(), sas.Identifier(), sas.Services(), sas.ResourceTypes(), sas.Signature())

	// You can then change some of the fields and construct a new URL:
	parts.SAS = azqueue.SASQueryParameters{} // Remove the SAS query parameters
	parts.QueueName = "otherqueue"           // Change the queue name
	parts.MessageID = ""

	// Construct a new URL from the parts:
	newURL, err := parts.URL()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(newURL.String())
	// NOTE: You can pass the new URL to NewQueueURL to manipulate the queue.

	// Output:
	// myaccount.queue.core.windows.net aqueue
	// 2015-02-21 q 2111-01-09 01:42:34.936 +0000 UTC 2222-03-09 01:42:34.936 +0000 UTC rup {168.1.5.60 168.1.5.70} https,http myIdentifier q o 92836758923659283652983562==
	// https://myaccount.queue.core.windows.net/otherqueue/messages
}

// This example shows how to create and use an Azure Storage account Shared Access Signature (SAS).
func ExampleAccountSASSignatureValues() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is required to sign a SAS.
	credential, err := azqueue.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}

	// Set the desired SAS signature values and sign them with the shared key credentials to get the SAS query parameters.
	sasQueryParams, err := azqueue.AccountSASSignatureValues{
		Protocol:      azqueue.SASProtocolHTTPS,       // Users MUST use HTTPS (not HTTP)
		ExpiryTime:    time.Now().Add(48 * time.Hour), // 48-hours before expiration
		Permissions:   azqueue.AccountSASPermissions{Read: true, List: true}.String(),
		Services:      azqueue.AccountSASServices{Queue: true}.String(),
		ResourceTypes: azqueue.AccountSASResourceTypes{Object: true}.String(),
	}.NewSASQueryParameters(credential)
	if err != nil {
		log.Fatal(err)
	}

	qp := sasQueryParams.Encode()
	urlToSendToSomeone := fmt.Sprintf("https://%s.queue.core.windows.net?%s", accountName, qp)
	// At this point, you can send the urlToSendToSomeone to someone via email or any other mechanism you choose.

	// ************************************************************************************************

	// When someone receives the URL, they access the SAS-protected resource with code like this:
	u, _ := url.Parse(urlToSendToSomeone)

	// Create an ServiceURL object that wraps the service URL (and its SAS) and a pipeline.
	// When using a SAS URLs, anonymous credentials are required.
	serviceURL := azqueue.NewServiceURL(*u, azqueue.NewPipeline(azqueue.NewAnonymousCredential(), azqueue.PipelineOptions{}))
	// Now, you can use this serviceURL just like any other to make requests of the resource.

	// You can parse a URL into its constituent parts:
	queueURLParts := azqueue.NewQueueURLParts(serviceURL.URL())
	fmt.Printf("SAS Protocol=%v\n", queueURLParts.SAS.Protocol())
	fmt.Printf("SAS Permissions=%v\n", queueURLParts.SAS.Permissions())
	fmt.Printf("SAS Services=%v\n", queueURLParts.SAS.Services())
	fmt.Printf("SAS ResourceTypes=%v\n", queueURLParts.SAS.ResourceTypes())

	// Output:
	// SAS Protocol=https
	// SAS Permissions=rl
	// SAS Services=q
	// SAS ResourceTypes=o
}

// This example shows how to create and use a Queue Service Shared Access Signature (SAS).
func ExampleQueueSASSignatureValues() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is required to sign a SAS.
	credential, err := azqueue.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}

	// This is the name of the queue that we're creating a SAS to.
	queueName := "queue4" // Queue names require lowercase

	// Set the desired SAS signature values and sign them with the shared key credentials to get the SAS query parameters.
	sasQueryParams := azqueue.QueueSASSignatureValues{
		Protocol:    azqueue.SASProtocolHTTPS,       // Users MUST use HTTPS (not HTTP)
		ExpiryTime:  time.Now().Add(48 * time.Hour), // 48-hours before expiration
		QueueName:   queueName,
		Permissions: azqueue.QueueSASPermissions{Add: true, Read: true, Process: true}.String(),
	}.NewSASQueryParameters(credential)

	// Create the URL of the resource you wish to access and append the SAS query parameters.
	// Since this is a queue SAS, the URL is to the Azure storage queue.
	qp := sasQueryParams.Encode()
	urlToSendToSomeone := fmt.Sprintf("https://%s.queue.core.windows.net/%s?%s",
		accountName, queueName, qp)
	// At this point, you can send the urlToSendToSomeone to someone via email or any other mechanism you choose.

	// ************************************************************************************************

	// When someone receives the URL, they access the SAS-protected resource with code like this:
	u, _ := url.Parse(urlToSendToSomeone)

	// Create an QueueURL object that wraps the queue URL (and its SAS) and a pipeline.
	// When using URls with a SAS, anonymous credentials are required.
	queueURL := azqueue.NewQueueURL(*u, azqueue.NewPipeline(azqueue.NewAnonymousCredential(), azqueue.PipelineOptions{}))
	// Now, you can use this queueURL just like any other to make requests of the resource.

	// If you have a SAS query parameter string, you can parse it into its parts:
	queueURLParts := azqueue.NewQueueURLParts(queueURL.URL())
	fmt.Printf("SAS Protocol=%v\n", queueURLParts.SAS.Protocol())
	fmt.Printf("SAS Permissions=%v\n", queueURLParts.SAS.Permissions())

	// Output:
	// SAS Protocol=https
	// SAS Permissions=rap
}

// This examples shows how to create a queue with metadata and then how to get properties & update the queue's metadata.
func ExampleQueueURL_GetProperties() {
	// From the Azure portal, get your Storage account file service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a FileURL with default pipeline based on an existing share with name myshare.
	u, _ := url.Parse(fmt.Sprintf("https://%s.queue.core.windows.net/queue5", accountName))
	credential, err := azqueue.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}
	queueURL := azqueue.NewQueueURL(*u, azqueue.NewPipeline(credential, azqueue.PipelineOptions{}))

	ctx := context.TODO() // This example uses a never-expiring context

	// Create a queue with metadata (string key/value pairs)
	// NOTE: Metadata key names are always converted to lowercase before being sent to the Storage Service.
	// Therefore, you should always use lowercase letters; especially when querying a map for a metadata key.
	_, err = queueURL.Create(ctx, azqueue.Metadata{"createdby": "Jeffrey"})
	if err != nil {
		log.Fatal(err)
	}

	// Query the queue's properties (which includes its metadata)
	props, err := queueURL.GetProperties(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Show some of the queue's read-only properties (0 messages in the queue)
	fmt.Println("Msg count=" + strconv.Itoa(int(props.ApproximateMessagesCount())))

	// Show the queue's metadata
	fmt.Println("Queue metadata values:")
	metadata := props.NewMetadata()
	for k, v := range metadata {
		fmt.Print("   " + k + "=" + v + "\n")
	}

	// Add a key/value pair to the queue's metadata
	metadata["updatedby"] = "Aidan" // Add a new key/value; NOTE: The keyname is in all lowercase letters
	_, err = queueURL.SetMetadata(ctx, metadata)
	if err != nil {
		log.Fatal(err)
	}

	props, err = queueURL.GetProperties(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Show the queue's metadata
	fmt.Println("Queue new metadata values:")
	for k, v := range props.NewMetadata() {
		fmt.Print("   " + k + "=" + v + "\n")
	}

	_, err = queueURL.Delete(ctx) // Delete the queue
	if err != nil {
		log.Fatal(err)
	}
	// Unordered output:
	// Msg count=0
	// Queue metadata values:
	//    createdby=Jeffrey
	// Queue new metadata values:
	//    createdby=Jeffrey
	//    updatedby=Aidan
}

// This examples shows how to clear all of a queue's messages.
func ExampleMessagesURL_Clear() {
	// From the Azure portal, get your Storage account file service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create a QueueURL with default pipeline based on an existing queue with name queue6.
	u, _ := url.Parse(fmt.Sprintf("https://%s.queue.core.windows.net/queue6", accountName))

	credential, err := azqueue.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.TODO() // This example uses a never-expiring context
	queueURL := azqueue.NewQueueURL(*u, azqueue.NewPipeline(credential, azqueue.PipelineOptions{}))
	_, err = queueURL.Create(ctx, azqueue.Metadata{})
	if err != nil {
		log.Fatal(err)
	}

	// Insert a message so we can verify it disappears after clearing the queue
	messagesURL := queueURL.NewMessagesURL()
	_, err = messagesURL.Enqueue(ctx, "A message", time.Second*0, time.Minute)
	if err != nil {
		log.Fatal(err)
	}

	props, err := queueURL.GetProperties(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Msg count=" + strconv.Itoa(int(props.ApproximateMessagesCount())))

	for {
		clear, err := messagesURL.Clear(ctx)
		if err == nil {
			break // Don't loop if Clear successful
		} else {
			if clear.StatusCode() == http.StatusInternalServerError {
				if stgErr, ok := err.(azqueue.StorageError); ok && stgErr.Response().StatusCode == http.StatusInternalServerError && stgErr.ServiceCode() == azqueue.ServiceCodeOperationTimedOut {
					continue // Service timed out while deleting messages; call Clear again until it return success
				}
			}
			log.Fatal(err)
		}
	}
	props, err = queueURL.GetProperties(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Msg count=" + strconv.Itoa(int(props.ApproximateMessagesCount())))

	// Output:
	// Msg count=1
	// Msg count=0
}

// This example shows how to set a queue's access policies.
func ExampleQueueURL_SetAccessPolicy() {
	// From the Azure portal, get your Storage account's name and account key.
	accountName, accountKey := accountInfo()

	// Use your Storage account's name and key to create a credential object; this is used to access your account.
	credential, err := azqueue.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}

	// Create a QueueURL object that wraps the queue's URL and a default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.queue.core.windows.net/queue7", accountName))
	queueURL := azqueue.NewQueueURL(*u, azqueue.NewPipeline(credential, azqueue.PipelineOptions{}))

	// All operations allow you to specify a timeout via a Go context.Context object.
	ctx := context.TODO() // This example uses a never-expiring context

	// Create the container (with no metadata)
	_, err = queueURL.Create(ctx, azqueue.Metadata{})
	if err != nil {
		log.Fatal(err)
	}

	// Create a URL that references a to-be-created blob in your Azure Storage account's container.
	// This returns a BlockBlobURL object that wraps the blob's URL and a request pipeline (inherited from containerURL)
	_, err = queueURL.SetAccessPolicy(ctx, []azqueue.SignedIdentifier{
		{
			ID:           "Managers",
			AccessPolicy: azqueue.AccessPolicy{Permission: azqueue.AccessPolicyPermission{Add: true}.String()},
		},
		{
			ID:           "Engineers",
			AccessPolicy: azqueue.AccessPolicy{Permission: azqueue.AccessPolicyPermission{Read: true, ProcessMessages: true}.String()},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	ap, err := queueURL.GetAccessPolicy(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Display the access policy's signedIdentifiers
	for _, si := range ap.Items {
		fmt.Println(si.ID, si.AccessPolicy.Permission)
	}

	if _, err := queueURL.Delete(ctx); err != nil { // Delete the queue that this example used
		log.Fatal(err)
	}

	// Output:
	// Managers a
	// Engineers rp
}

// This examples shows how to list the queues in an Azure Storage service account.
func ExampleServiceURL_ListQueuesSegment() {
	// From the Azure portal, get your Storage account file service URL endpoint.
	accountName, accountKey := accountInfo()

	// Create an account ServiceURL with default pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.queue.core.windows.net/", accountName))
	credential, err := azqueue.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		log.Fatal(err)
	}
	serviceURL := azqueue.NewServiceURL(*u, azqueue.NewPipeline(credential,
		azqueue.PipelineOptions{}))

	ctx := context.TODO() // This example uses a never-expiring context

	// Create a few queues (with metadata) so that we can list them:
	q1 := serviceURL.NewQueueURL("zqueue-1")
	if _, err := q1.Create(ctx, azqueue.Metadata{"key": "value"}); err != nil {
		log.Fatal(err)
	}

	q2 := serviceURL.NewQueueURL("zqueue-2")
	_, err = q2.Create(ctx, azqueue.Metadata{"k": "v"})
	if err != nil {
		log.Fatal(err)
	}

	// List the queues in the account; since an account may hold millions of queues, this is done 1 segment at a time.
	for marker := (azqueue.Marker{}); marker.NotDone(); { // The parens around Marker{} are required to avoid compiler error.
		// Get a result segment starting with the queue indicated by the current Marker.
		segmentResponse, err := serviceURL.ListQueuesSegment(ctx, marker,
			azqueue.ListQueuesSegmentOptions{
				Prefix: "zqueue",
				Detail: azqueue.ListQueuesSegmentDetails{Metadata: true},
			})
		if err != nil {
			log.Fatal(err)
		}
		// IMPORTANT: ListQueuesSegment returns the start of the next segment; you MUST use this to get
		// the next segment (after processing the current result segment).
		marker = segmentResponse.NextMarker

		// Process the queues returned in this segment (if the segment is empty, the loop body won't execute)
		for _, queueItem := range segmentResponse.QueueItems {
			fmt.Print("Queue name: " + queueItem.Name + "\n")
			for k, v := range queueItem.Metadata {
				fmt.Printf("   k=%v, v=%v\n", k, v)
			}
		}
	}

	// Delete the queues we created earlier.
	if _, err = q1.Delete(ctx); err != nil {
		log.Fatal(err)
	}
	if _, err = q2.Delete(ctx); err != nil {
		log.Fatal(err)
	}

	// Output:
	// Queue name: zqueue-1
	//    k=key, v=value
	// Queue name: zqueue-2
	//    k=k, v=v
}
