# Azure Storage Queue SDK for Go
[![GoDoc Widget]][GoDoc] [![Build Status][Travis Widget]][Travis]

The Microsoft Azure Storage SDK for Go allows you to build applications that takes advantage of Azure's scalable cloud storage. 

This repository contains the open source Queue SDK for Go.

## Features
* Queue Storage
	* Create/List/Delete Queues
	* Enqueue/Update/Delete queue messages

## Getting Started
* If you don't already have it, install [the Go distribution](https://golang.org/dl/)
* Get the SDK, with any method you prefer:
    * Go Get: ```go get github.com/Azure/azure-storage-queue-go/azqueue```
    * Dep: add ```github.com/Azure/azure-storage-queue-go``` to Gopkg.toml:
        ```
        [[constraint]]
          version = "0.2.0"
          name = "github.com/Azure/azure-storage-queue-go"
        ```
    * Module: simply import the SDK and Go will download it for you
* Use the SDK:
```import "github.com/Azure/azure-storage-queue-go/azqueue"```

## Version Table
* If you are looking to use a specific version of the Storage Service, please refer to the following table: 

| Service Version | Corresponding SDK Version | Import Path                                                |
|-----------------|---------------------------|------------------------------------------------------------|
| 2017-07-29      | 0.1.0                     | github.com/Azure/azure-storage-queue-go/2017-07-29/azqueue |
| 2018-03-28      | 0.2.0                     | github.com/Azure/azure-storage-queue-go/azqueue            |
|                 |                           |                                                            |

## SDK Architecture
* The Azure Storage SDK for Go provides APIs that map 1-to-1 with the 
[Azure Storage Queue REST APIs](https://docs.microsoft.com/en-us/rest/api/storageservices/queue-service-rest-api) via
 the ServiceURL, QueueURL, MessagesURL, and MessageIDURL types.

## Code Samples
* [Queue Storage Examples](https://godoc.org/github.com/Azure/azure-storage-queue-go/2017-07-29/azqueue#pkg-examples)

## License
This project is licensed under MIT.

## Contributing
This project welcomes contributions and suggestions.  Most contributions require you to agree to a
Contributor License Agreement (CLA) declaring that you have the right to, and actually do, grant us
the rights to use your contribution. For details, visit https://cla.microsoft.com.

When you submit a pull request, a CLA-bot will automatically determine whether you need to provide
a CLA and decorate the PR appropriately (e.g., label, comment). Simply follow the instructions
provided by the bot. You will only need to do this once across all repos using our CLA.

This project has adopted the [Microsoft Open Source Code of Conduct](https://opensource.microsoft.com/codeofconduct/).
For more information see the [Code of Conduct FAQ](https://opensource.microsoft.com/codeofconduct/faq/) or
contact [opencode@microsoft.com](mailto:opencode@microsoft.com) with any additional questions or comments.

[GoDoc]: https://godoc.org/github.com/Azure/azure-storage-queue-go/2017-07-29/azqueue
[GoDoc Widget]: https://godoc.org/github.com/Azure/azure-storage-queue-go/2017-07-29/azqueue?status.svg
[Travis]: https://travis-ci.org/Azure/azure-storage-queue-go
[Travis Widget]: https://travis-ci.org/Azure/azure-storage-queue-go.svg?branch=master
