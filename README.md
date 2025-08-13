# eventsourcingdb

The official Go client SDK for [EventSourcingDB](https://www.eventsourcingdb.io) – a purpose-built database for event sourcing.

EventSourcingDB enables you to build and operate event-driven applications with native support for writing, reading, and observing events. This client SDK provides convenient access to its capabilities in Go.

For more information on EventSourcingDB, see its [official documentation](https://docs.eventsourcingdb.io/).

This client SDK includes support for [Testcontainers](https://testcontainers.com/) to spin up EventSourcingDB instances in integration tests. For details, see [Using Testcontainers](#using-testcontainers).

## Getting Started

Install the client SDK:

```shell
go get github.com/thenativeweb/eventsourcingdb-client-golang
```

Import the package and create an instance by providing the URL of your EventSourcingDB instance and the API token to use:

```go
import (
  "net/url"

  "github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
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
```

Then call the `Ping` function to check whether the instance is reachable. If it is not, the function will return an error:

```go
err := client.Ping()
if err != nil {
  // ...
}
```

*Note that `Ping` does not require authentication, so the call may succeed even if the API token is invalid.*

If you want to verify the API token, call `VerifyAPIToken`. If the token is invalid, the function will return an error:

```go
err := client.VerifyAPIToken()
if err != nil {
  // ...
}
```

### Writing Events

Call the `WriteEvents` function and hand over a slice with one or more events. You do not have to provide all event fields – some are automatically added by the server.

Specify `Source`, `Subject`, `Type`, and `Data` according to the [CloudEvents](https://docs.eventsourcingdb.io/fundamentals/cloud-events/) format.

For `Data`, you may provide any struct with public fields. It is recommended to have JSON annotations on this struct to control how the struct gets serialized.

The function returns the written events, including the fields added by the server:

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
  []eventsourcingdb.EventCandidate{
    event,
  },
  nil,
)
```

#### Using the `isSubjectPristine` precondition

If you only want to write events in case a subject (such as `/books/42`) does not yet have any events, use the `NewIsSubjectPristinePrecondition` function to create a precondition and pass it in a slice as the second argument:

```go
writtenEvents, err := client.WriteEvents(
  []eventsourcingdb.EventCandidate{
    // ...
  },
  []eventsourcingdb.Precondition{
    eventsourcingdb.NewIsSubjectPristinePrecondition("/books/42"),
  },
)
```

#### Using the `isSubjectOnEventId` precondition

If you only want to write events in case the last event of a subject (such as `/books/42`) has a specific ID (e.g., `0`), use the `NewIsSubjectOnEventIDPrecondition` function to create a precondition and pass it in a slice as the second argument:

```go
writtenEvents, err := client.WriteEvents(
  []eventsourcingdb.EventCandidate{
    // ...
  },
  []eventsourcingdb.Precondition{
    eventsourcingdb.NewIsSubjectOnEventIDPrecondition("/books/42", "0"),
  },
)
```

*Note that according to the CloudEvents standard, event IDs must be of type string.*

#### Using the `isEventQlQueryTrue` precondition

If you want to write events depending on an EventQL query, use the `NewIsEventQLQueryTruePrecondition` function to create a precondition:

```go
writtenEvents, err := client.WriteEvents(
  []eventsourcingdb.EventCandidate{
    // ...
  },
  []eventsourcingdb.Precondition{
    eventsourcingdb.NewIsEventQLQueryTruePrecondition("FROM e IN events WHERE e.type == 'io.eventsourcingdb.library.book-borrowed' PROJECT INTO COUNT() < 10"),
  },
)
```

*Note that the query must return a single row with a single value, which is interpreted as a boolean.*

### Reading Events

To read all events of a subject, call the `ReadEvents` function with a context, the subject and an options object. Set the `Recursive` option to `false`. This ensures that only events of the given subject are returned, not events of nested subjects.

The function returns an iterator, which you can use e.g. inside a `for range` loop:

```golang
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

#### Reading From Subjects Recursively

If you want to read not only all the events of a subject, but also the events of all nested subjects, set the `Recursive` option to `true`:

```golang
for event, err := range client.ReadEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ReadEventsOptions{
    Recursive: true,
  },
) {
  // ...
}
```

This also allows you to read *all* events ever written. To do so, provide `/` as the subject and set `Recursive` to `true`, since all subjects are nested under the root subject.

#### Reading in Anti-Chronological Order

By default, events are read in chronological order. To read in anti-chronological order, provide the `Order` option and set it using the `OrderAntichronological` function:

```golang
for event, err := range client.ReadEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ReadEventsOptions{
    Recursive: false,
    Order:     eventsourcingdb.OrderAntichronological(),
  },
) {
  // ...
}
```

*Note that you can also use the `OrderChronological` function to explicitly enforce the default order.*

#### Specifying Bounds

Sometimes you do not want to read all events, but only a range of events. For that, you can specify the `LowerBound` and `UpperBound` options – either one of them or even both at the same time.

Specify the ID and whether to include or exclude it, for both the lower and upper bound:

```golang
for event, err := range client.ReadEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ReadEventsOptions{
    Recursive:  false,
    LowerBound: &eventsourcingdb.Bound{
      ID:   "100",
      Type: eventsourcingdb.BoundTypeInclusive,
    },
    UpperBound: &eventsourcingdb.Bound{
      ID:   "200",
      Type: eventsourcingdb.BoundTypeExclusive,
    },
  },
) {
  // ...
}
```

#### Starting From the Latest Event of a Given Type

To read starting from the latest event of a given type, provide the `FromLatestEvent` option and specify the subject, the type, and how to proceed if no such event exists.

Possible options are `ReadNothingIfEventIsMissing`, which skips reading entirely, or `ReadEverythingIfEventIsMissing`, which effectively behaves as if `FromLatestEvent` was not specified:

```golang
for event, err := range client.ReadEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ReadEventsOptions{
    Recursive:  false,
    FromLatestEvent: &eventsourcingdb.ReadFromLatestEvent{
      Subject:          "/books/42",
      Type:             "io.eventsourcingdb.library.book-borrowed",
      IfEventIsMissing: eventsourcingdb.ReadEverythingIfEventIsMissing,
    },
  },
) {
  // ...
}
```

*Note that `FromLatestEvent` and `LowerBound` can not be provided at the same time.*

#### Aborting Reading

If you need to abort reading use `break` or `return` within the `for range` loop. However, this only works if there is currently an iteration going on.

To abort reading independently of that, cancel the context you provided:

```golang
ctx, cancel := context.WithCancel(context.TODO())

for event, err := range client.ReadEvents(
  ctx,
  "/books/42",
  eventsourcingdb.ReadEventsOptions{
    Recursive:  false,
  },
) {
  // ...
}

// Somewhere else, cancel the context, which will cause
// reading to end.
cancel()
```

### Running EventQL Queries

To run an EventQL query, call the `RunEventQLQuery` function and provide a context and the query as arguments. The function returns an iterator, which you can use e.g. inside a `for range` loop:

```golang
for row, err := range client.RunEventQLQuery(
  context.TODO(),
  "FROM e IN events PROJECT INTO e",
) {
  // ...
}
```

*Note that each row returned by the iterator is of type `json.RawMessage` and matches the projection specified in your query.*

#### Aborting a Query

If you need to abort a query use `break` or `return` within the `for range` loop. However, this only works if there is currently an iteration going on.

To abort the query independently of that, cancel the context you provided:

```golang
ctx, cancel := context.WithCancel(context.TODO())

for row, err := range client.RunEventQLQuery(
  ctx,
  "FROM e IN events PROJECT INTO e",
) {
  // ...
}

// Somewhere else, cancel the context, which will cause
// the query to end.
cancel()
```

### Observing Events

To observe all events of a subject, call the `ObserveEvents` function with a context, the subject and an options object. Set the `Recursive` option to `false`. This ensures that only events of the given subject are returned, not events of nested subjects.

The function returns an iterator, which you can use e.g. inside a `for range` loop:

```golang
for event, err := range client.ObserveEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive: false,
  },
) {
  // ...
}
```

#### Observing From Subjects Recursively

If you want to observe not only all the events of a subject, but also the events of all nested subjects, set the `Recursive` option to `true`:

```golang
for event, err := range client.ObserveEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive: true,
  },
) {
  // ...
}
```

This also allows you to observe *all* events ever written. To do so, provide `/` as the subject and set `Recursive` to `true`, since all subjects are nested under the root subject.

#### Specifying Bounds

Sometimes you do not want to observe all events, but only a range of events. For that, you can specify the `LowerBound` option.

Specify the ID and whether to include or exclude it:

```golang
for event, err := range client.ObserveEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive:  false,
    LowerBound: &eventsourcingdb.Bound{
      ID:   "100",
      Type: eventsourcingdb.BoundTypeInclusive,
    },
  },
) {
  // ...
}
```

#### Starting From the Latest Event of a Given Type

To observe starting from the latest event of a given type, provide the `FromLatestEvent` option and specify the subject, the type, and how to proceed if no such event exists.

Possible options are `WaitForEventIfEventIsMissing`, which waits for an event of the given type to happen, or `ObserveEverythingIfEventIsMissing`, which effectively behaves as if `FromLatestEvent` was not specified:

```golang
for event, err := range client.ObserveEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive:  false,
    FromLatestEvent: &eventsourcingdb.ObserveFromLatestEvent{
      Subject:          "/books/42",
      Type:             "io.eventsourcingdb.library.book-borrowed",
      IfEventIsMissing: eventsourcingdb.ObserveEverythingIfEventIsMissing,
    },
  },
) {
  // ...
}
```

*Note that `FromLatestEvent` and `LowerBound` can not be provided at the same time.*

#### Aborting Observing

If you need to abort observing use `break` or `return` within the `for range` loop. However, this only works if there is currently an iteration going on.

To abort observing independently of that, cancel the context you provided:

```golang
ctx, cancel := context.WithCancel(context.TODO())

for event, err := range client.ObserveEvents(
  ctx,
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive:  false,
  },
) {
  // ...
}

// Somewhere else, cancel the context, which will cause
// observing to end.
cancel()
```

### Registering an Event Schema

To register an event schema, call the `RegisterEventSchema` function and hand over an event type and the desired schema:

```golang
client.RegisterEventSchema(
  "io.eventsourcingdb.library.book-acquired",
  map[string]any{
    "type": "object",
    "properties": map[string]any{
      "title":  map[string]any{ "type": "string" },
      "author": map[string]any{ "type": "string" },
      "isbn":   map[string]any{ "type": "string" },
    },
    "required": []string{
      "title",
      "author",
      "isbn",
    },
    "additionalProperties": false,
  },
)
```

### Listing Subjects

To list all subjects, call the `ReadSubjects` function with a context and `/` as the base subject. The function returns an iterator, which you can use e.g. inside a `for range` loop:

```golang
for subject, err := range client.ReadSubjects(
  context.TODO(),
  "/",
) {
  // ...
}
```

If you only want to list subjects within a specific branch, provide the desired base subject instead:

```golang
for subject, err := range client.ReadSubjects(
  context.TODO(),
  "/books",
) {
  // ...
}
```

#### Aborting Listing

If you need to abort listing use `break` or `return` within the `for range` loop. However, this only works if there is currently an iteration going on.

To abort listing independently of that, cancel the context you provided:

```golang
ctx, cancel := context.WithCancel(context.TODO())

for subject, err := range client.ReadSubjects(
  ctx,
  "/",
) {
  // ...
}

// Somewhere else, cancel the context, which will cause
// reading to end.
cancel()
```

### Listing Event Types

To list all event types, call the `ReadEventTypes` function. The function returns an iterator, which you can use e.g. inside a `for range` loop:

```golang
for eventType, err := range client.ReadEventTypes(
  context.TODO(),
) {
  // ...
}
```

#### Aborting Listing

If you need to abort listing use `break` or `return` within the `for range` loop. However, this only works if there is currently an iteration going on.

To abort listing independently of that, cancel the context you provided:

```golang
ctx, cancel := context.WithCancel(context.TODO())

for eventType, err := range client.ReadEventTypes(
  context.TODO(),
) {
  // ...
}

// Somewhere else, cancel the context, which will cause
// reading to end.
cancel()
```

### Listing a Specific Event Type

To list a specific event type, call the `ReadEventType` function with the event type as an argument. The function returns the detailed event type, which includes the schema:

```golang
eventType, err := client.ReadEventType(
  "io.eventsourcingdb.library.book-acquired",
)
```

### Verifying an Event's Hash

To verify the integrity of an event, call the `VerifyHash` function on the event instance. This recomputes the event's hash locally and compares it to the hash stored in the event. If the hashes differ, the function returns an error:

```golang
err := event.VerifyHash()
if err != nil {
  // ...
}
```

*Note that this only verifies the hash. If you also want to verify the signature, you can skip this step and call `VerifySignature` directly, which performs a hash verification internally.*

### Verifying an Event's Signature

To verify the authenticity of an event, call the `VerifySignature` function on the event instance. This requires the public key that matches the private key used for signing on the server.

The function first verifies the event's hash, and then checks the signature. If any verification step fails, it returns an error:

```golang
import "crypto/ed25519"

// ...

verificationKey := /* public key as ed25519.PublicKey */

err := event.VerifySignature(verificationKey)
if err != nil {
  // ...
}
```

### Using Testcontainers

Call the `NewContainer` function, start the test container, defer stopping it, get a client, and run your test code:

```go
ctx := context.TODO()

container := eventsourcingdb.NewContainer()
container.Start(ctx)
defer container.Stop(ctx)

client, err := container.GetClient(ctx)
if err != nil {
  // ...
}

// ...
```

To check if the test container is running, call the `IsRunning` function:

```go
isRunning := container.IsRunning()
```

#### Configuring the Container Instance

By default, `Container` uses the `latest` tag of the official EventSourcingDB Docker image. To change that, call the `WithImageTag` function:

```go
container := eventsourcingdb.NewContainer().
  WithImageTag("1.0.0")
```

Similarly, you can configure the port to use and the API token. Call the `WithPort` or the `WithAPIToken` function respectively:

```go
container := eventsourcingdb.NewContainer().
  WithPort(4000).
  WithAPIToken("secret")
```

#### Configuring the Client Manually

In case you need to set up the client yourself, use the following functions to get details on the container:

- `GetHost()` returns the host name
- `GetMappedPort()` returns the port
- `GetBaseURL()` returns the full URL of the container
- `GetAPIToken()` returns the API token
