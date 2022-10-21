package test

type WithAuthorization struct {
	ContainerizedTestingDatabase
	AccessToken string
}

type WithoutAuthorization struct {
	ContainerizedTestingDatabase
}

type WithInvalidURL struct {
	TestingDatabase
}

type Database struct {
	WithAuthorization    WithAuthorization
	WithoutAuthorization WithoutAuthorization
	WithInvalidURL       WithInvalidURL
}
