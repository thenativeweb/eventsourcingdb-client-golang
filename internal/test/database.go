package test

type Database struct {
	WithAuthorization    ContainerizedTestingDatabase
	WithoutAuthorization ContainerizedTestingDatabase
	WithInvalidURL       TestingDatabase
}
