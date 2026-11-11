# eventsourcingdb

The official Go client SDK for [EventSourcingDB](https://www.eventfoundation.io/products/eventsourcingdb) – a purpose-built database for event sourcing.

The complete documentation of this client SDK, including preconditions, EventQL, observing events, handling errors, Testcontainers, and upgrading, is available at **[https://www.eventfoundation.io/docs/eventsourcingdb/sdks/go](https://www.eventfoundation.io/docs/eventsourcingdb/sdks/go)**.

## Getting Started

Install the client SDK:

```shell
go get github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb
```

Create a client with the URL of your EventSourcingDB instance and the API token to use, and check whether the instance is reachable:

```go
import (
  "context"
  "net/url"

  "github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb"
)

// ...

baseURL, err := url.Parse("http://localhost:3000")
if err != nil {
  // ...
}

apiToken := "secret"

client, err := eventsourcingdb.NewClient(baseURL, apiToken)
if err != nil {
  // ...
}

err = client.Ping(context.TODO())
if err != nil {
  // ...
}
```

Write an event:

```go
type BookAcquired struct {
  Title  string `json:"title"`
  Author string `json:"author"`
  ISBN   string `json:"isbn"`
}

event := eventsourcingdb.EventCandidate{
  Source:  "https://library.eventsourcingdb.io",
  Subject: "/books/42",
  Type:    "io.eventsourcingdb.library.book-acquired",
  Data: BookAcquired{
    Title:  "2001 – A Space Odyssey",
    Author: "Arthur C. Clarke",
    ISBN:   "978-0756906788",
  },
}

writtenEvents, err := client.WriteEvents(
  context.TODO(),
  []eventsourcingdb.EventCandidate{
    event,
  },
  nil,
)
```

Read the events of a subject:

```go
for event, err := range client.ReadEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ReadEventsOptions{
    Recursive: false,
  },
) {
  // ...
}
```

For everything else, see the **[complete documentation](https://www.eventfoundation.io/docs/eventsourcingdb/sdks/go)**.
