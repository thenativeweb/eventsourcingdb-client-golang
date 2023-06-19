package test

type Database struct {
	WithAuthorization ContainerizedTestingDatabase
	WithInvalidURL    TestingDatabase
}
