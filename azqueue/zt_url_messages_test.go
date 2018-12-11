package azqueue_test

import (
	"github.com/Azure/azure-storage-queue-go/azqueue"
	chk "gopkg.in/check.v1"
	"time"
)

// verify the normal operations of enqueueing messages
func validateEnqueue(c *chk.C, messagesURL azqueue.MessagesURL, messageContent string, visibilityTimeout, timeToLive time.Duration) {
	resp, err := messagesURL.Enqueue(ctx, messageContent, visibilityTimeout, timeToLive)
	c.Assert(err, chk.Equals, nil)
	c.Assert(resp.StatusCode(), chk.Equals, 201)
	c.Assert(resp.MessageID, chk.Not(chk.Equals), nil)
	c.Assert(resp.PopReceipt, chk.Not(chk.Equals), nil)

	// make sure that the time intervals were submitted properly
	// TimeNextVisible - InsertionTime must be equal to the visibilityTimeout that we submitted
	c.Assert(resp.TimeNextVisible.Sub(resp.InsertionTime).Seconds(), chk.Equals, visibilityTimeout.Seconds())

	// avoid testing the default value (0) or infinite value (-1)
	// ExpirationTime - InsertionTime must be equal to the timeToLive that we submitted
	if timeToLive > 0 {
		c.Assert(resp.ExpirationTime.Sub(resp.InsertionTime).Seconds(), chk.Equals, timeToLive.Seconds())
	}
}

// validate error cases of calling enqueue
// make sure the right errors are thrown
func validateEnqueueError(c *chk.C, messagesURL azqueue.MessagesURL, messageContent string, visibilityTimeout, timeToLive time.Duration, expectedErrorCode string) {
	_, err := messagesURL.Enqueue(ctx, messageContent, visibilityTimeout, timeToLive)
	c.Assert(err, chk.Not(chk.Equals), nil)

	// if an error from the service is expected, validate the error code
	if expectedErrorCode != "" {
		// make sure the error can be casted to StorageError
		storageErr := err.(azqueue.StorageError)
		c.Assert(storageErr.ServiceCode(), chk.Equals, azqueue.ServiceCodeType(expectedErrorCode))
	}
}

func (s *queueSuite) TestEnqueue(c *chk.C) {
	// setup
	qsu, _ := getGenericQueueServiceURL()
	queueURL, _ := createNewQueue(c, qsu)
	defer deleteQueue(c, queueURL)
	messagesURL := queueURL.NewMessagesURL()

	// basic scenario
	validateEnqueue(c, messagesURL, "testContent", 0, 0)

	// non-zero timeToLive
	validateEnqueue(c, messagesURL, "testContent", 0, 5 * time.Second)

	// non-zero visibilityTimeout
	validateEnqueue(c, messagesURL, "testContent", 7 * time.Minute, 0)

	// non-zero timeToLive and visibilityTimeout
	validateEnqueue(c, messagesURL, "testContent", 5 * time.Second, 7 * time.Hour)

	// error case: negative visibilityTimeout
	// this is a validation error so we do not pass any expectedErrorCode
	validateEnqueueError(c, messagesURL, "testContent", -time.Second, 0, "")

	// error case: negative timeToLive
	// this is a validation error so we do not pass any expectedErrorCode
	validateEnqueueError(c, messagesURL, "testContent", 0, -time.Hour, "")

	// error case: use a non-existing queue and attempt to enqueue
	queueURL2, _ := getQueueURL(qsu)
	messagesURL2 := queueURL2.NewMessagesURL()
	validateEnqueueError(c, messagesURL2, "testContent", 0, 0, "QueueNotFound")
}
