# eventsourcingdb

The official Go client SDK for [EventSourcingDB](https://www.eventsourcingdb.io) – a purpose-built database for event sourcing.

EventSourcingDB enables you to build and operate event-driven applications with native support for writing, reading, and observing events. This client SDK provides convenient access to its capabilities in Go.

For more information on EventSourcingDB, see its [official documentation](https://docs.eventsourcingdb.io/).

This client SDK includes support for [Testcontainers](https://testcontainers.com/) to spin up EventSourcingDB instances in integration tests. It lives in a package of its own, so applications that only use the client do not depend on Testcontainers. For details, see [Using Testcontainers](#using-testcontainers).

If you upgrade from version 1 of this client SDK, see [Upgrading From Version 1](#upgrading-from-version-1).

## Getting Started

Install the client SDK:

```shell
go get github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb
```

Import the package and create an instance by providing the URL of your EventSourcingDB instance and the API token to use:

```go
import (
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
```

*Note that the client sends its requests with an HTTP client of its own, which starts from a copy of `http.DefaultTransport`. Changing `http.DefaultClient`, e.g. setting its `Timeout`, does not affect it.*

Then call the `Ping` function with a context to check whether the instance is reachable. If it is not, the function will return an error:

```go
err := client.Ping(context.TODO())
if err != nil {
  // ...
}
```

*Note that `Ping` does not require authentication, so the call may succeed even if the API token is invalid.*

If you want to verify the API token, call `VerifyAPIToken` with a context. If the token is invalid, the function will return an error:

```go
err := client.VerifyAPIToken(context.TODO())
if err != nil {
  // ...
}
```

### Talking to Servers With and Without the v2 API

The client works with servers that offer the v2 API of EventSourcingDB, and with servers that offer the v1 API only. Creating a client sends no request. On its first request, the client asks the server whether it offers the v2 API, and remembers the answer: it speaks the v2 API if the server offers it, and the v1 API otherwise. If the server gives no clear answer, e.g. because it can not be reached, that first request fails with the error, and the next request asks again. Since the client asks with `GET /api/v2/system/ping`, the error of that first request then names that path, whatever the request was, e.g. `Get "http://localhost:3000/api/v2/system/ping": dial tcp [::1]:3000: connect: connection refused`.

*Note that with the v2 API, the client reads with the HTTP method `QUERY` ([RFC 10008](https://www.rfc-editor.org/rfc/rfc10008)). If a proxy runs in front of the database, it has to let `QUERY` through. The client does not fall back to `POST`.*

### Configuring the Stall Timeout

A request fails if nothing arrives from the server for 30 seconds, e.g. because the server can not be reached, or because a proxy keeps the connection open but no longer passes anything on (see [Handling Errors](#handling-errors)). To change this stall timeout, pass the `WithStallTimeout` option to `NewClient`:

```go
client, err := eventsourcingdb.NewClient(
  baseURL,
  apiToken,
  eventsourcingdb.WithStallTimeout(10*time.Second),
)
if err != nil {
  // ...
}
```

The clock starts when a request is sent, so it covers connecting and waiting for the response, and starts again with every byte that arrives. It does not limit how long a request takes in total: a stream that keeps receiving events or heartbeats never ends because of it, and the time you spend on an item of a stream does not count. The stall timeout must be greater than zero, and since the server sends a heartbeat every second while observing events and running EventQL queries, it should be several seconds at least. To limit a single request further, give its context a deadline.

### Writing Events

Call the `WriteEvents` function and hand over a context and a slice with one or more events. You do not have to provide all event fields – some are automatically added by the server.

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
  context.TODO(),
  []eventsourcingdb.EventCandidate{
    event,
  },
  nil,
)
```

#### Using the `isSubjectPristine` precondition

If you only want to write events in case a subject (such as `/books/42`) does not yet have any events, use the `NewIsSubjectPristinePrecondition` function to create a precondition and pass it in a slice as the third argument:

```go
writtenEvents, err := client.WriteEvents(
  context.TODO(),
  []eventsourcingdb.EventCandidate{
    // ...
  },
  []eventsourcingdb.Precondition{
    eventsourcingdb.NewIsSubjectPristinePrecondition("/books/42"),
  },
)
```

#### Using the `isSubjectPopulated` precondition

If you only want to write events in case a subject (such as `/books/42`) already has at least one event, use the `NewIsSubjectPopulatedPrecondition` function to create a precondition and pass it in a slice as the third argument:

```go
writtenEvents, err := client.WriteEvents(
  context.TODO(),
  []eventsourcingdb.EventCandidate{
    // ...
  },
  []eventsourcingdb.Precondition{
    eventsourcingdb.NewIsSubjectPopulatedPrecondition("/books/42"),
  },
)
```

#### Using the `isSubjectOnEventId` precondition

If you only want to write events in case the last event of a subject (such as `/books/42`) has a specific ID (e.g., `0`), use the `NewIsSubjectOnEventIDPrecondition` function to create a precondition and pass it in a slice as the third argument:

```go
writtenEvents, err := client.WriteEvents(
  context.TODO(),
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
  context.TODO(),
  []eventsourcingdb.EventCandidate{
    // ...
  },
  []eventsourcingdb.Precondition{
    eventsourcingdb.NewIsEventQLQueryTruePrecondition("FROM e IN events WHERE e.type == 'io.eventsourcingdb.library.book-borrowed' PROJECT INTO COUNT() < 10"),
  },
)
```

*Note that the query must return a single row with a single value, which is interpreted as a boolean.*

#### Aborting Writing

To abort writing, e.g. if the server does not answer in time, cancel the context you provided, or give it a deadline:

```go
ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
defer cancel()

writtenEvents, err := client.WriteEvents(
  ctx,
  []eventsourcingdb.EventCandidate{
    // ...
  },
  nil,
)
```

Writing then ends with the error of the context, `context.Canceled` or `context.DeadlineExceeded`. If the context ends before the client has connected to the server, no events are written. If it ends later, the server may have written them nevertheless, so the error wraps the error of the context with `ErrOutcomeUnknown` (see [Handling Errors](#handling-errors)).

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

Possible options are `ReadIfEventIsMissingReadNothing`, which skips reading entirely, or `ReadIfEventIsMissingReadEverything`, which effectively behaves as if `FromLatestEvent` was not specified:

```golang
for event, err := range client.ReadEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ReadEventsOptions{
    Recursive:  false,
    FromLatestEvent: &eventsourcingdb.ReadFromLatestEvent{
      Subject:          "/books/42",
      Type:             "io.eventsourcingdb.library.book-borrowed",
      IfEventIsMissing: eventsourcingdb.ReadIfEventIsMissingReadEverything,
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

Reading then ends with the error of the context, `context.Canceled`, so that an aborted read can be told apart from a complete one.

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

The query then ends with the error of the context, `context.Canceled`, so that an aborted query can be told apart from a complete one.

#### Detecting a Stalled Connection

While a query runs, the server sends a heartbeat every second whenever there is no row to send. If neither a row nor a heartbeat arrives for the stall timeout, 30 seconds by default, e.g. because a proxy keeps the connection open but no longer passes anything on, the query ends with `ErrConnectionStalled`, as every other request does, and the client closes the connection (see [Handling Errors](#handling-errors)). Use `errors.Is` to check for it:

```golang
for row, err := range client.RunEventQLQuery(
  context.TODO(),
  "FROM e IN events PROJECT INTO e",
) {
  if errors.Is(err, eventsourcingdb.ErrConnectionStalled) {
    // ...
  }

  // ...
}
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

Possible options are `ObserveIfEventIsMissingWaitForEvent`, which waits for an event of the given type to happen, or `ObserveIfEventIsMissingReadEverything`, which effectively behaves as if `FromLatestEvent` was not specified:

```golang
for event, err := range client.ObserveEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive:  false,
    FromLatestEvent: &eventsourcingdb.ObserveFromLatestEvent{
      Subject:          "/books/42",
      Type:             "io.eventsourcingdb.library.book-borrowed",
      IfEventIsMissing: eventsourcingdb.ObserveIfEventIsMissingReadEverything,
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

Observing then ends with the error of the context, `context.Canceled`. Since observing does not end on its own, this is how it usually ends. If the server ends the stream nevertheless, e.g. after a failure it does not report, observing ends with an error that says that the stream broke off, since you may have missed events. A server that shuts down reports that as a `DBAPIError` with the code `CodeServerShuttingDown`.

#### Detecting a Stalled Connection

While there are no new events, the server sends a heartbeat every second. If neither an event nor a heartbeat arrives for the stall timeout, 30 seconds by default, e.g. because a proxy keeps the connection open but no longer passes anything on, observing ends with `ErrConnectionStalled`, as every other request does, and the client closes the connection (see [Handling Errors](#handling-errors)). Use `errors.Is` to check for it:

```golang
for event, err := range client.ObserveEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive: false,
  },
) {
  if errors.Is(err, eventsourcingdb.ErrConnectionStalled) {
    // ...
  }

  // ...
}
```

### Registering an Event Schema

To register an event schema, call the `RegisterEventSchema` function and hand over a context, an event type and the desired schema:

```golang
client.RegisterEventSchema(
  context.TODO(),
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

If a schema is already registered for the event type, the function returns a `DBAPIError` with the code `CodeEventSchemaAlreadyRegistered`, and if events of that type were written before that do not match the schema, one with the code `CodeEventSchemaViolated`. As with writing events, an error that leaves open whether the schema was registered wraps its cause with `ErrOutcomeUnknown` (see [Handling Errors](#handling-errors)).

### Listing Subjects

To list subjects, call the `ReadSubjects` function with a context, a subject and an options object. Set the `Recursive` option to `true` to list the subject together with all subjects nested below it, or to `false` to list the subject alone, if it exists. There is no default, so set it either way. To list all subjects, provide `/` and set `Recursive` to `true`. The function returns an iterator, which you can use e.g. inside a `for range` loop:

```golang
for subject, err := range client.ReadSubjects(
  context.TODO(),
  "/",
  eventsourcingdb.ReadSubjectsOptions{
    Recursive: true,
  },
) {
  // ...
}
```

If you only want to list subjects within a specific branch, provide the subject of that branch instead:

```golang
for subject, err := range client.ReadSubjects(
  context.TODO(),
  "/books",
  eventsourcingdb.ReadSubjectsOptions{
    Recursive: true,
  },
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
  eventsourcingdb.ReadSubjectsOptions{
    Recursive: true,
  },
) {
  // ...
}

// Somewhere else, cancel the context, which will cause
// reading to end.
cancel()
```

Reading then ends with the error of the context, `context.Canceled`, so that an aborted read can be told apart from a complete one.

### Listing Event Types

To list all event types, call the `ReadEventTypes` function. The function returns an iterator, which you can use e.g. inside a `for range` loop. Each `EventType` has the name of the event type as `Type`, tells with `HasEvents` whether events of that type were written, and has the registered schema as `Schema`, or `nil` if there is none:

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
  ctx,
) {
  // ...
}

// Somewhere else, cancel the context, which will cause
// reading to end.
cancel()
```

Reading then ends with the error of the context, `context.Canceled`, so that an aborted read can be told apart from a complete one.

### Listing a Specific Event Type

To list a specific event type, call the `ReadEventType` function with a context and the event type as arguments. The function returns the detailed event type, which includes the schema:

```golang
eventType, err := client.ReadEventType(
  context.TODO(),
  "io.eventsourcingdb.library.book-acquired",
)
```

If the database knows nothing of the event type, neither events nor a schema, the function returns a `DBAPIError` with the code `CodeEventTypeUnknown`.

### Verifying an Event's Hash

To verify the integrity of an event, call the `VerifyHash` function on the event instance. This recomputes the event's hash locally and compares it to the hash stored in the event. If the hashes differ, the function returns `ErrHashVerificationFailed`. Use `errors.Is` to check for it:

```golang
err := event.VerifyHash()
if errors.Is(err, eventsourcingdb.ErrHashVerificationFailed) {
  // ...
}
```

The hash of an event tells which version of the hash it is. EventSourcingDB writes version 2 since version 1.3, which starts with `esdb:hash:v2:` and also covers the `TraceParent` and the `TraceState` of an event. Events written before keep their hash of version 1, which has no prefix. `VerifyHash` supports both, so you can verify every event of a store.

If the hash is of a version the client does not know yet, e.g. one that starts with `esdb:hash:v3:`, `VerifyHash` can not tell whether it matches. It then returns an error that wraps `ErrUnsupportedHashVersion` and names the version, e.g. `unsupported hash version 'v3'`. This does not mean that the event was changed, but that you need a newer version of the client to verify it. Use `errors.Is` to check for it:

```golang
err := event.VerifyHash()
if errors.Is(err, eventsourcingdb.ErrUnsupportedHashVersion) {
  // ...
}
```

*Note that this only verifies the hash. If you also want to verify the signature, you can skip this step and call `VerifySignature` directly, which performs a hash verification internally.*

### Verifying an Event's Signature

To verify the authenticity of an event, call the `VerifySignature` function on the event instance. This requires the public key that matches the private key used for signing on the server.

The function first verifies the event's hash, and then checks the signature. If any verification step fails, it returns an error: `ErrHashVerificationFailed` if the hash does not match, `ErrUnsupportedHashVersion` if the hash is of a version the client does not know, and `ErrSignatureVerificationFailed` if the signature does not match. Use `errors.Is` to check for them:

```golang
import "crypto/ed25519"

// ...

verificationKey := /* public key as ed25519.PublicKey */

err := event.VerifySignature(verificationKey)
if errors.Is(err, eventsourcingdb.ErrSignatureVerificationFailed) {
  // ...
}
```

### Handling Errors

Every error that the server reports is a `*DBAPIError`, so you can tell failures apart without parsing the error message. Use `errors.As` to get it. It has these fields:

- `Code` identifies the error with the URL of the page that documents it, exactly as the server sends it, e.g. `https://docs.eventsourcingdb.io/errors/precondition-violated`. The constants that start with `Code` name every code there is, e.g. `CodePreconditionViolated`. `Code` is empty for an error without a code, e.g. a token that the server does not accept.
- `Description` describes the error, as the server words it.
- `Details` locates the error and names the value at fault, as the JSON object the server sends with a code, e.g. `{"precondition":0}`, or `nil`.
- `Status` is the HTTP status of the answer. It is `200` for an error that the server reports in the body of its answer, e.g. within a stream, and e.g. `401` for a token that it does not accept.
- `RetryAfter` is how long the server asks to wait before sending the request again, for status `429`, as its `Retry-After` header says. The client never sends a request again on its own.
- `Action` names what the client tried to do, e.g. `write events`.

Tell failures apart by their code rather than by their status:

```golang
_, err := client.WriteEvents(
  context.TODO(),
  []eventsourcingdb.EventCandidate{
    // ...
  },
  []eventsourcingdb.Precondition{
    eventsourcingdb.NewIsSubjectPristinePrecondition("/books/42"),
  },
)

var dbAPIError *eventsourcingdb.DBAPIError
if errors.As(err, &dbAPIError) {
  switch dbAPIError.Code {
  case eventsourcingdb.CodePreconditionViolated:
    // dbAPIError.Description tells which precondition does not hold.
  case eventsourcingdb.CodeEventSchemaViolated:
    // ...
  }

  if dbAPIError.Status == http.StatusTooManyRequests {
    // Wait for dbAPIError.RetryAfter before trying again.
  }
}
```

*Note that a server with the v1 API only sends no codes. The client gives those of its errors that clearly match an error of the v2 API the code of that error, e.g. `CodePreconditionViolated` for a precondition that does not hold, and leaves the code of every other one empty.*

If writing events or registering an event schema fails in a way that leaves open whether the server stored it nevertheless, the error wraps its cause with `ErrOutcomeUnknown`. This is the case, e.g., if the connection stalls or breaks after the client has connected, if your context ends after that, if a stream breaks off, or if the server fails unexpectedly. Every other error of a write means that the server certainly did not store anything: either the client could not connect to it at all, or the server refused the write, e.g. because a precondition does not hold. Use `errors.Is` to check for it, which also finds the cause, as does `errors.As`:

```golang
_, err := client.WriteEvents(
  context.TODO(),
  []eventsourcingdb.EventCandidate{
    // ...
  },
  nil,
)

if errors.Is(err, eventsourcingdb.ErrOutcomeUnknown) {
  // Read the events to find out whether they were written, or write them
  // again with a precondition that does not hold if they were.
}
```

If a stream that ends on its own breaks off before it is complete, or if the server ends observing events, which never ends on its own, the stream ends with an error that says that it broke off, since the items it handed out may not be all of them.

If the server is not an EventSourcingDB, e.g. because a proxy answers in its place, the client returns `ErrInvalidServerHeader`. Use `errors.Is` to check for it:

```golang
err := client.Ping(context.TODO())
if errors.Is(err, eventsourcingdb.ErrInvalidServerHeader) {
  // ...
}
```

If nothing arrives from the server for the stall timeout, 30 seconds by default (see [Configuring the Stall Timeout](#configuring-the-stall-timeout)), e.g. because the server can not be reached, or because a proxy keeps the connection open but no longer passes anything on, the client closes the connection, and the request ends with `ErrConnectionStalled`. This applies to every request, also to streams once the server has begun to answer, on which heartbeats count as something that arrives. Its message names the stall timeout. Use `errors.Is` to check for it:

```golang
for event, err := range client.ObserveEvents(
  context.TODO(),
  "/books/42",
  eventsourcingdb.ObserveEventsOptions{
    Recursive: false,
  },
) {
  if errors.Is(err, eventsourcingdb.ErrConnectionStalled) {
    // ...
  }

  // ...
}
```

*Note that a write that stalls once the client has connected to the server wraps `ErrConnectionStalled` with `ErrOutcomeUnknown`, since the server may have stored it nevertheless. A write that stalls before that, e.g. while the client connects, or while it asks the server which API it offers on its first request, returns `ErrConnectionStalled` itself, since the server certainly did not store it.*

Since a stalled connection is a timeout, `errors.Is` also finds `context.DeadlineExceeded` in the error, as it does for a timeout of an `http.Client`. To tell a stall from a deadline of your context, check for `ErrConnectionStalled`, or check whether your context has ended.

### Using Testcontainers

The test container lives in the `eventsourcingdbtest` package. Add it to your module, and import it in your tests:

```shell
go get github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest
```

```go
import "github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest"
```

Call the `NewContainer` function, start the test container, defer stopping it, get a client, and run your test code:

```go
ctx := context.TODO()

container := eventsourcingdbtest.NewContainer()
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
container := eventsourcingdbtest.NewContainer().
  WithImageTag("1.0.0")
```

Similarly, you can configure the port to use and the API token. Call the `WithPort` or the `WithAPIToken` function respectively:

```go
container := eventsourcingdbtest.NewContainer().
  WithPort(4000).
  WithAPIToken("secret")
```

If you want to sign events, call the `WithSigningKey` function. This generates a new signing and verification key pair inside the container:

```go
container := eventsourcingdbtest.NewContainer().
  WithSigningKey()
```

You can retrieve the private key (for signing) and the public key (for verifying signatures) once the container has been started:

```go
signingKey, err := container.GetSigningKey()
verificationKey, err := container.GetVerificationKey()
```

The `signingKey` is the private key EventSourcingDB signs events with. The `verificationKey` can be passed to `VerifySignature` when verifying events read from the database.

#### Configuring the Client Manually

In case you need to set up the client yourself, use the following functions to get details on the container:

- `GetHost()` returns the host name
- `GetMappedPort()` returns the port
- `GetBaseURL()` returns the full URL of the container
- `GetAPIToken()` returns the API token

*Note that up to version 1.5, the test container was part of the `eventsourcingdb` package. To upgrade, import `eventsourcingdbtest` and call `eventsourcingdbtest.NewContainer` instead of `eventsourcingdb.NewContainer`.*

## Upgrading From Version 1

Version 2 of the client SDK speaks the v2 API of EventSourcingDB with servers that offer it, and the v1 API with all others (see [Talking to Servers With and Without the v2 API](#talking-to-servers-with-and-without-the-v2-api)). Upgrading takes these changes:

- **The module path ends with `/v2`.** Import `github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdb` instead of `github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb`, and `github.com/thenativeweb/eventsourcingdb-client-golang/v2/eventsourcingdbtest` instead of `github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdbtest`.
- **`ErrHeartbeatTimeout` is gone.** A stream that stalls once the server has begun to answer ends with `ErrConnectionStalled`, as every other request that stalls. Replace `errors.Is(err, eventsourcingdb.ErrHeartbeatTimeout)` with `errors.Is(err, eventsourcingdb.ErrConnectionStalled)`.
- **An event type names its type `Type`, and tells with `HasEvents` whether it has events.** `eventType.EventType` is now `eventType.Type`, and `eventType.IsPhantom` is now `!eventType.HasEvents`.
- **`ReadSubjects` takes options with a `Recursive` option, which you have to set.** `client.ReadSubjects(ctx, "/books")` is now `client.ReadSubjects(ctx, "/books", eventsourcingdb.ReadSubjectsOptions{Recursive: true})`, which lists what it listed before. With `Recursive: false`, it lists the subject alone.
- **`DBAPIError` has a `Status` and a `Description`, and reports every error of the server.** `dbAPIError.StatusCode` is now `dbAPIError.Status`, and `dbAPIError.Reason` is now `dbAPIError.Description`. The error message says `failed to read the description` where it said `failed to read the reason`. An error within a stream, e.g. of an EventQL query, used to be a plain error, and is a `DBAPIError` with status `200` now. With a server that offers the v2 API, an error that the server reports about the request itself has status `200` and a code, e.g. a precondition that does not hold, rather than status `409`. Check `dbAPIError.Code == eventsourcingdb.CodePreconditionViolated` instead of `dbAPIError.StatusCode == http.StatusConflict`, which works with both APIs.
- **A `DBAPIError` is no longer comparable**, as its `Details` is a `json.RawMessage`. Code that compares two values of type `DBAPIError` with `==`, or that uses one as the key of a map, no longer compiles. Compare their fields instead, e.g. `Code`. A `*DBAPIError`, as the client returns it, is still comparable.
- **A write that may have been stored wraps its error with `ErrOutcomeUnknown`.** `WriteEvents` and `RegisterEventSchema` used to return the cause as it was, e.g. `ErrConnectionStalled` or `context.DeadlineExceeded`. They now return an error that wraps it, so `errors.Is` and `errors.As` still find the cause, but the message starts with `outcome unknown, `.
- **Observing events fails if the server ends the stream.** `ObserveEvents` used to end without an error if the server closed the stream, and ends with an error that says that the stream broke off now.
- **With a server that offers the v2 API, reads use the HTTP method `QUERY`.** A proxy in front of the database has to let it through. There is no fallback to `POST`.
