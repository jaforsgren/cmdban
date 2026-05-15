package pat

type Entry struct {
	Name  string `yaml:"name"`
	Token string `yaml:"token"`
}

type Store struct {
	PATs     []Entry `yaml:"pats"`
	filePath string
}
