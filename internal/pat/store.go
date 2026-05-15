package pat

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const storeFileName = ".cmdban-pats.yaml"

func DefaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return storeFileName
	}
	return filepath.Join(home, storeFileName)
}

func Load() (*Store, error) {
	path := DefaultStorePath()
	store := &Store{filePath: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return store, nil
	}
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(data, store); err != nil {
		return nil, err
	}
	store.filePath = path
	return store, nil
}

func (s *Store) Save() error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(s.filePath, data, 0600)
}

func (s *Store) Get(name string) (string, bool) {
	for _, p := range s.PATs {
		if p.Name == name {
			return p.Token, true
		}
	}
	return "", false
}

func (s *Store) Set(name, token string) {
	for i, p := range s.PATs {
		if p.Name == name {
			s.PATs[i].Token = token
			return
		}
	}
	s.PATs = append(s.PATs, Entry{Name: name, Token: token})
}

func (s *Store) Delete(name string) {
	for i, p := range s.PATs {
		if p.Name == name {
			s.PATs = append(s.PATs[:i], s.PATs[i+1:]...)
			return
		}
	}
}
