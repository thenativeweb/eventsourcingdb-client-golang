# eventsourcingdb

The official Go client SDK for [EventSourcingDB](https://www.eventsourcingdb.io) – a purpose-built database for event sourcing.

EventSourcingDB enables you to build and operate event-driven applications with native support for writing, reading, and observing events. This client SDK provides convenient access to its capabilities in Go.

For more information on EventSourcingDB, see its [official documentation](https://docs.eventsourcingdb.io/).

This client SDK includes support for [Testcontainers](https://testcontainers.com/) to spin up EventSourcingDB instances in integration tests. For details, see [Using Testcontainers](#using-testcontainers).

## Getting Started

Import the package and create an instance by providing the URL of your EventSourcingDB instance and the API token to use:

```go
import (
  "net/url"

  "github.com/thenativeweb/eventsourcingdb-client-golang/eventsourcingdb"
)

// ...

baseUrl, err := url.Parse("http://localhost:3000")
if err != nil {
  // ...
}

apiToken := "secret"

client, err := eventsourcingdb.NewClient(baseUrl, apiToken)
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
  .WithPort(4000)
  .WithAPIToken("secret");
```

#### Configuring the Client Manually

In case you need to set up the client yourself, use the following functions to get details on the container:

- `GetHost()` returns the host name.
- `GetMappedPort()` returns the port.
- `GetBaseURL()` returns the full URL of the container.
- `GetAPIToken()` returns the API token.
