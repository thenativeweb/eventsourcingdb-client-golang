package eventsourcingdb

type Precondition interface {
	discriminator()
}
