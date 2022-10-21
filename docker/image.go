package docker

type Image struct {
	Name string
	Tag  string
}

func (image Image) GetFullName() string {
	return image.Name + ":" + image.Tag
}
