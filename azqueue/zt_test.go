package azqueue_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-storage-queue-go/azqueue"
	chk "gopkg.in/check.v1"
)

const (
	queuePrefix = "go"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { chk.TestingT(t) }

var ctx = context.Background()

type queueSuite struct{}

var _ = chk.Suite(&queueSuite{})

func getGenericCredential(accountType string) (*azqueue.SharedKeyCredential, error) {
	accountNameEnvVar := accountType + "ACCOUNT_NAME"
	accountKeyEnvVar := accountType + "ACCOUNT_KEY"
	accountName, accountKey := os.Getenv(accountNameEnvVar), os.Getenv(accountKeyEnvVar)
	if accountName == "" || accountKey == "" {
		// docker run -p 10000:10000 -p 10001:10001 mcr.microsoft.com/azure-storage/azurite
		// See https://docs.microsoft.com/en-us/azure/storage/common/storage-use-azurite
		accountName = "devstoreaccount1"
		accountKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="
	}
	return azqueue.NewSharedKeyCredential(accountName, accountKey)
}

func getGenericQueueServiceURL() (azqueue.ServiceURL, error) {
	credential, err := getGenericCredential("")
	if err != nil {
		return azqueue.ServiceURL{}, err
	}

	pipeline := azqueue.NewPipeline(credential, azqueue.PipelineOptions{})
	var blobPrimaryURL *url.URL
	if credential.AccountName() == "devstoreaccount1" {
		blobPrimaryURL, _ = url.Parse("http://127.0.0.1:10001/devstoreaccount1")
	} else {
		blobPrimaryURL, _ = url.Parse("https://" + credential.AccountName() + ".queue.core.windows.net/")
	}
	return azqueue.NewServiceURL(*blobPrimaryURL, pipeline), nil
}

// This function generates an entity name by concatenating the passed prefix,
// the name of the test requesting the entity name, and the minute, second, and nanoseconds of the call.
// This should make it easy to associate the entities with their test, uniquely identify
// them, and determine the order in which they were created.
// Note that this imposes a restriction on the length of test names
func generateName(prefix string) string {
	// These next lines up through the for loop are obtaining and walking up the stack
	// trace to extrat the test name, which is stored in name
	pc := make([]uintptr, 10)
	runtime.Callers(0, pc)
	f := runtime.FuncForPC(pc[0])
	name := f.Name()
	for i := 0; !strings.Contains(name, "Suite"); i++ { // The tests are all scoped to the suite, so this ensures getting the actual test name
		f = runtime.FuncForPC(pc[i])
		name = f.Name()
	}
	funcNameStart := strings.Index(name, "Test")
	name = name[funcNameStart+len("Test"):] // Just get the name of the test and not any of the garbage at the beginning
	name = strings.ToLower(name)            // Ensure it is a valid resource name
	currentTime := time.Now()
	name = fmt.Sprintf("%s%s%d%d%d", prefix, strings.ToLower(name), currentTime.Minute(), currentTime.Second(), currentTime.Nanosecond())
	return name
}

func generateQueueName() string {
	return generateName(queuePrefix)
}

func getQueueURL(qsu azqueue.ServiceURL) (queue azqueue.QueueURL, name string) {
	name = generateQueueName()
	queue = qsu.NewQueueURL(name)
	return
}

func createNewQueue(c *chk.C, qsu azqueue.ServiceURL) (queue azqueue.QueueURL, name string) {
	queue, name = getQueueURL(qsu)

	cResp, err := queue.Create(ctx, nil)
	c.Assert(err, chk.IsNil)
	c.Assert(cResp.StatusCode(), chk.Equals, 201)
	return queue, name
}

func deleteQueue(c *chk.C, qsu azqueue.QueueURL) {
	resp, err := qsu.Delete(ctx)
	c.Assert(err, chk.IsNil)
	c.Assert(resp.StatusCode(), chk.Equals, 204)
}

/*
Add 204 to Create Queue success status codes

Call delete on non-existant queue
Set access condition and try op that always fails
//Bad credentials for SAS or shared key
test failing ACL
time to live - test that we send this properly
visibility timeout
*/
