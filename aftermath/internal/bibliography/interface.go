package bibliography

type Entry struct {
	Target string
	Title  string
	Path   string
}

type Bibliography interface {
	Append([]Entry) error
	Override([]Entry) error
}
