# eventsourcing-client-golang

The Go client for EventSourcingDB.

## Quick start

First, import the module:

```golang
import (
  "github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
  "github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb/event"
)
```

### Creating a client

To create a new client, call the `eventsourcingdb.NewClient` function and specify the URL of your EventSourcingDB instance as well as the access token:

```golang
client, err := eventsourcingdb.NewClient(
  "http://localhost:3000",
  "secret",
)
```

Optionally, you may specify the number of retries to perform in case of a failure:

```golang
client, err := eventsourcingdb.NewClient(
  "http://localhost:3000",
  "secret",
  eventsourcingdb.MaxTries(3),
)
```

### Veryfying the connection

To verify the connection to EventSourcingDB, call the `client.Ping` function:

```golang
err := client.Ping()
```

### Writing events

Before writing events, you probably want to create a source which represents your application. For that, call the `event.NewSource` function and specify the name of your application:

```golang
source := event.NewSource(
  "tag:thenativeweb.io,2023:auth",
)
```

Then you can start creating events by calling the `source.NewEvent` function and specify the subject of the event as well as the type of the event:

```golang
type UserRegistered struct {
  Login    string `json:"login"`
  Password string `json:"password"`
}

userRegistered := source.NewEvent(
  "/user/23",
  "io.thenativeweb.user.registered",
  UserRegistered{
    Login:    "janedoe",
    Password: "secret",
  },
)
```

Finally, you can write the event by calling the `client.WriteEvents` function and passing the event. You may also write multiple events at once:

```golang
written, err := client.WriteEvents(
  []event.Candidate{
    userRegistered,
  },
)
```

In `written`, you will find the details of the events that were written.

#### Using preconditions

Optionally, you may specify preconditions which must be fulfilled for the events to be written. One option is to ensure that the subject is pristine, i.e. that no events have been written for it yet:

```golang
precondition := eventsourcingdb.IsSubjectPristine("/user/23")
```

Another option is to ensure that the subject is on a specific event ID:

```golang
precondition := eventsourcingdb.IsSubjectOnEventID("/user/23", "42")
```

Either way, hand over your preconditions as additional parameters to the `WriteEvents` function:

```golang
written, err := client.WriteEvents(
  []event.Candidate{
    userRegistered,
  },
  precondition,
)
```

### Validating events with JSON schemas

To validate events with JSON schemas, use the `client.RegisterEventSchema` function and specify the type of the event you want to create a schema for as well as the JSON schema:

```golang
err = client.RegisterEventSchema(
  "io.thenativeweb.user.registered",
  `{
    "type": "object",
    "properties": {
      "login": { "type": "string" },
      "password": { "type": "string" }
    },
    "required": [ "login", "password" ],
    "additionalProperties": false
  }`,
)
```

### Reading events

To read events, call the `client.ReadEvents` function and specify the subject of the events you want to read as well as whether you want to read recursively (`eventsourcingdb.ReadRecursively`) or non-recursively (`eventsourcingdb.ReadNonRecursively`):

```golang
results, err := client.ReadEvents(
  context.TODO(),
  "/user/23",
  eventsourcingdb.ReadNonRecursively(),
)
```

The return value is a channel that you can iterate over. Each item in this channel can be an event or an error. You may use the `result.IsData` and `result.IsError` functions to determine the type of the item:

```golang
for result := range results {
  fmt.Println(result.IsData())
  fmt.Println(result.IsError())
}
```

Alternatively, you may call the `result.GetData` function to get the event or the error:

```golang
for result := range results {
  hashedEvent, err := result.GetData()
  if err != nil {
    // ...
  }

  fmt.Println(hashedEvent.Hash)
  fmt.Println(hashedEvent.Event)
}
```

To access the event's data, you need to unmarshal the event's `Data` property:

```golang
userRegistered := &UserRegistered{}
err := json.Unmarshal(hashedEvent.Event.Data, userRegistered)
if err != nil {
  // ...
}

fmt.Println(userRegistered.Login, userRegistered.Password)
```

#### Using read options

Optionally, you may specify further options for reading events as additional parameters to the `client.ReadEvents` function.

To change the order in which events are read, specify `eventsourcingdb.ReadChronologically` or `eventsourcingdb.ReadAntichronologically`:

```golang
results := client.ReadEvents(
  context.TODO(),
  "/user/23",
  eventsourcingdb.ReadNonRecursively(),
  eventsourcingdb.ReadChronologically(),
)
```

You also may specify a lower or upper bound for the event ID, i.e. the event ID from which to start reading events or the event ID up to which to read events. For that, use the functions `eventsourcingdb.ReadFromLowerBoundID` and `eventsourcingdb.ReadUntilUpperBoundID` respectively:

```golang
results := client.ReadEvents(
  context.TODO(),
  "/user/23",
  eventsourcingdb.ReadNonRecursively(),
  eventsourcingdb.ReadFromLowerBoundID("42"),
  eventsourcingdb.ReadUntilUpperBoundID("65"),
)
```

Finally, you may also specify to read from the latest event of a given type by using `eventsourcingdb.ReadFromLatestEvent`. For that, you also have to provide the subject, the event type, and what to do if the event is missing (either `ifeventismissingduringread.ReadNothing` or `ifeventismissingduringread.ReadEverything`):

```golang
results := client.ReadEvents(
  context.TODO(),
  "/user/23",
  eventsourcingdb.ReadNonRecursively(),
  eventsourcingdb.ReadFromLatestEvent(
    "/user/23",
    "io.thenativeweb.user.registered",
    ifeventismissingduringread.ReadEverything,
  ),
)
```

### Observing events

To observe events, call the `client.ObserveEvents` function and specify the subject of the events you want to observe as well as whether you want to observe recursively (`eventsourcingdb.ObserveRecursively`) or non-recursively (`eventsourcingdb.ObserveNonRecursively`):

```golang
results := client.ObserveEvents(
  context.TODO(),
  "/user/23",
  eventsourcingdb.ObserveNonRecursively(),
)
```

The return value is a channel that you can iterate over. Each item in this channel can be an event or an error. You may use the `result.IsData` and `result.IsError` functions to determine the type of the item:

```golang
for result := range results {
  fmt.Println(result.IsData())
  fmt.Println(result.IsError())
}
```

Alternatively, you may call the `result.GetData` function to get the event or the error:

```golang
for result := range results {
  hashedEvent, err := result.GetData()
  if err != nil {
    // ...
  }

  fmt.Println(hashedEvent.Hash)
  fmt.Println(hashedEvent.Event)
}
```

To access the event's data, you need to unmarshal the event's `Data` property:

```golang
userRegistered := &UserRegistered{}
err := json.Unmarshal(hashedEvent.Event.Data, userRegistered)
if err != nil {
  // ...
}

fmt.Println(userRegistered.Login, userRegistered.Password)
```

#### Using observe options

Optionally, you may specify further options for observing events as additional parameters to the `client.ObserveEvents` function.

You may specify a lower bound for the event ID, i.e. the event ID from which to start observing events. For that, use the function `eventsourcingdb.ObserveFromLowerBoundID`:

```golang
results := client.ObserveEvents(
  context.TODO(),
  "/user/23",
  eventsourcingdb.ObserveNonRecursively(),
  eventsourcingdb.ObserveFromLowerBoundID("42"),
)
```

Additionally, you may also specify to observe from the latest event of a given type by using `eventsourcingdb.ObserveFromLatestEvent`. For that, you also have to provide the subject, the event type, and what to do if the event is missing (either `ifeventismissingduringobserve.ReadEverything` or `ifeventismissingduringobserve.WaitForEvent`):

```golang
results := client.ObserveEvents(
  context.TODO(),
  "/user/23",
  eventsourcingdb.ReadNonRecursively(),
  eventsourcingdb.ReadFromLatestEvent(
    "/user/23",
    "io.thenativeweb.user.registered",
    ifeventismissingduringobserve.ReadEverything,
  ),
)
```

### Reading subjects

To read subjects, call the `client.ReadSubjects` function:

```golang
results, err := client.ReadSubjects(
  context.TODO(),
)
```

The return value is a channel that you can iterate over. Each item in this channel can be a subject or an error. You may use the `result.IsData` and `result.IsError` functions to determine the type of the item:

```golang
for result := range results {
  fmt.Println(result.IsData())
  fmt.Println(result.IsError())
}
```

Alternatively, you may call the `result.GetData` function to get the subject or the error:

```golang
for result := range results {
  subject, err := result.GetData()
  if err != nil {
    // ...
  }

  fmt.Println(subject)
}
```

Optionally, you may specify a base subject to read subjects from:

```golang
results, err := client.ReadSubjects(
  context.TODO(),
  eventsourcingdb.BaseSubject("/user"),
)
```

### Reading event types

To read subjects, call the `client.ReadEventTypes` function:

```golang
results, err := client.ReadEventTypes(
  context.TODO(),
)
```

The return value is a channel that you can iterate over. Each item in this channel can be an event type or an error. You may use the `result.IsData` and `result.IsError` functions to determine the type of the item:

```golang
for result := range results {
  fmt.Println(result.IsData())
  fmt.Println(result.IsError())
}
```

Alternatively, you may call the `result.GetData` function to get the event type or the error:

```golang
for result := range results {
  eventType, err := result.GetData()
  if err != nil {
    // ...
  }

  fmt.Println(eventType)
}
```

## Running quality assurance

To run quality assurance for this module use the following command:

```shell
$ make
```
