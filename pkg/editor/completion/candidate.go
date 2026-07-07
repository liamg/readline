package completion

type Group struct {
	Name       string
	Candidates []Candidate
}

type Candidate struct {
	Name        string
	Description string
	Content     string // actual content to write to the buffer when confirmed
	Join        string // extra stuff to add once tab is pressed again after selecting this candidate e.g. "/" after a dir
}
