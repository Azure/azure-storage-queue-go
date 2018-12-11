package azqueue_test

import (
	"github.com/Azure/azure-storage-queue-go/azqueue"
	chk "gopkg.in/check.v1"
)

func (s *queueSuite) TestCreateDeleteQueue(c *chk.C) {
	qsu, _ := getGenericQueueServiceURL()

	// create a queue
	queueURL, queueName := createNewQueue(c, qsu)
	c.Assert(queueURL, chk.Not(chk.Equals), nil)
	c.Assert(queueName, chk.Not(chk.Equals), nil)

	// validate that queue was successfully created
	resp, err := queueURL.GetProperties(ctx)
	c.Assert(err, chk.Equals, nil)
	c.Assert(resp, chk.Not(chk.Equals), nil)

	// clean up
	deleteQueue(c, queueURL)
}

func (s *queueSuite) TestGetPropertiesOnNonExistentQueue(c *chk.C) {
	qsu, _ := getGenericQueueServiceURL()
	queueURL, _ := getQueueURL(qsu)

	// validate that queue does not exist
	_, err := queueURL.GetProperties(ctx)
	c.Assert(err, chk.Not(chk.Equals), nil)

	// cast to StorageError and validate
	storageErr := err.(azqueue.StorageError)
	c.Assert(storageErr.ServiceCode(), chk.Equals, azqueue.ServiceCodeType("QueueNotFound"))
	c.Assert(storageErr.Response().StatusCode, chk.Equals, 404)
}
