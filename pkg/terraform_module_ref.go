package pkg

import (
	"fmt"
	"github.com/Azure/mapotf/pkg/terraform"
	"github.com/go-git/go-git/v5"
	"os"
	"path/filepath"
)

type TerraformModuleRef struct {
	Key     string `json:"Key"`
	Source  string `json:"Source"`
	Dir     string `json:"Dir"`
	AbsDir  string
	Version string `json:"Version"`
	GitHash string
}

func NewTerraformRootModuleRef(dir string) (*TerraformModuleRef, error) {
	return NewTerraformModuleRef(dir, "", "", "")
}

func NewTerraformModuleRef(dir, key, source, version string) (*TerraformModuleRef, error) {
	m := &TerraformModuleRef{
		Key:     key,
		Source:  source,
		Dir:     dir,
		Version: version,
	}
	if err := m.LoadAbsDir(); err != nil {
		return nil, err
	}
	m.LoadGitHash()
	return m, nil
}

func (m *TerraformModuleRef) Load() error {
	if err := m.LoadAbsDir(); err != nil {
		return err
	}
	m.LoadGitHash()
	return nil
}

func (m *TerraformModuleRef) LoadGitHash() {
	h, err := gitHash(m.AbsDir)
	if err != nil {
		//TODO:log error
		return
	}
	m.GitHash = h
}

func (m *TerraformModuleRef) LoadAbsDir() error {
	absDir, err := filepath.Abs(m.Dir)
	if err != nil {
		return fmt.Errorf("error getting absolute path for %s: %+v", m.Dir, err)
	}
	m.AbsDir = absDir
	return nil
}

func (r *TerraformModuleRef) toTerraformPkgType() terraform.TerraformModuleRef {
	return terraform.TerraformModuleRef{
		Key:     r.Key,
		Source:  r.Source,
		Dir:     r.Dir,
		AbsDir:  r.AbsDir,
		Version: r.Version,
		GitHash: r.GitHash,
	}
}

func gitHash(dir string) (string, error) {
	gitPath, err := lookupGitPath(dir)
	if err != nil {
		return "", fmt.Errorf("cannot lookup git path: %+v", err)
	}
	r, err := git.PlainOpen(filepath.Dir(gitPath))
	if err != nil {
		return "", err
	}
	ref, err := r.Head()
	if err != nil {
		return "", err
	}
	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return "", err
	}
	return commit.Hash.String(), nil
}

func lookupGitPath(path string) (string, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(filepath.Join(path, ".git"))
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		isBare, err := isBareRepo(path)
		if err != nil {
			return "", err
		}
		if isBare {
			return path, nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf(".git not found")
		}
		return lookupGitPath(parent)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf(".git exist but is not a directory")
	}
	return filepath.Join(path, ".git"), nil
}

func isBareRepo(path string) (bool, error) {
	markers := []string{"HEAD", "objects", "refs"}
	for _, marker := range markers {
		_, err := os.Stat(filepath.Join(path, marker))
		if err != nil && !os.IsNotExist(err) {
			return false, err
		}
		if err != nil {
			return false, nil
		}
	}

	return true, nil
}
