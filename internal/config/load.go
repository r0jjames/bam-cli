package config

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/r0jjames/bam-cli/internal/errs"
	"gopkg.in/yaml.v3"
)

// ProjectFileName is the committed per-repository config file.
const ProjectFileName = ".bam.yaml"

// Env holds the BAM_* environment variables that affect server selection.
type Env struct {
	Server string // BAM_SERVER
	URL    string // BAM_URL
	Token  string // BAM_TOKEN
}

// Config is both layers as loaded, before merging.
type Config struct {
	Project     *ProjectFile // nil when the repository has no .bam.yaml
	Machine     *MachineFile // never nil
	ProjectPath string
	MachinePath string
	RepoRoot    string // git root, or the working directory outside a repository
	Env         Env
}

type LoadOptions struct {
	WorkDir     string
	Home        string
	MachineFile string
	Getenv      func(string) string
}

// Discover finds .bam.yaml by walking up from workDir. The walk stops at the
// git root inside a repository, else at home. repoRoot is the git root, or
// workDir outside a repository.
func Discover(workDir, home string) (projectPath, repoRoot string, err error) {
	workDir, err = filepath.Abs(workDir)
	if err != nil {
		return "", "", err
	}
	gitRoot := findUp(workDir, "", ".git")
	stop := gitRoot
	repoRoot = gitRoot
	if gitRoot == "" {
		repoRoot = workDir
		stop = workDir
		if home != "" && within(workDir, filepath.Clean(home)) {
			stop = filepath.Clean(home)
		}
	}
	if dir := findUp(workDir, stop, ProjectFileName); dir != "" {
		projectPath = filepath.Join(dir, ProjectFileName)
	}
	return projectPath, repoRoot, nil
}

// findUp returns the first directory from start upwards that contains name.
// It never goes above stop when stop is set.
func findUp(start, stop, name string) string {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return dir
		}
		if dir == stop {
			return ""
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Load discovers and parses both layers and reads the BAM_* variables.
func Load(o LoadOptions) (*Config, error) {
	projectPath, root, err := Discover(o.WorkDir, o.Home)
	if err != nil {
		return nil, errs.Configf("cannot resolve the working directory").Wrap(err)
	}
	cfg := &Config{ProjectPath: projectPath, MachinePath: o.MachineFile, RepoRoot: root}
	cfg.Env = Env{Server: o.Getenv("BAM_SERVER"), URL: o.Getenv("BAM_URL"), Token: o.Getenv("BAM_TOKEN")}

	if projectPath != "" {
		data, err := os.ReadFile(projectPath)
		if err != nil {
			return nil, errs.Configf("cannot read %s", projectPath).Wrap(err)
		}
		if err := checkProjectCredentialKeys(data, projectPath); err != nil {
			return nil, err
		}
		var pf ProjectFile
		if err := decodeStrict(data, projectPath, &pf); err != nil {
			return nil, err
		}
		if err := checkVersion(pf.Version, projectPath); err != nil {
			return nil, err
		}
		cfg.Project = &pf
	}

	cfg.Machine = &MachineFile{Version: 1}
	if o.MachineFile != "" {
		data, err := os.ReadFile(o.MachineFile)
		switch {
		case errors.Is(err, fs.ErrNotExist):
		case err != nil:
			return nil, errs.Configf("cannot read %s", o.MachineFile).Wrap(err)
		default:
			var mf MachineFile
			if err := decodeStrict(data, o.MachineFile, &mf); err != nil {
				return nil, err
			}
			if err := checkVersion(mf.Version, o.MachineFile); err != nil {
				return nil, err
			}
			cfg.Machine = &mf
		}
	}
	if err := validateFiles(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func decodeStrict(data []byte, path string, out any) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return errs.Configf("%s is empty", path).WithTry("add a line: version: 1")
	}
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return errs.Configf("invalid %s", path).WithWhy(err.Error()).Wrap(err)
	}
	return nil
}

func checkVersion(v int, path string) error {
	if v == 0 {
		return errs.Configf("missing version in %s", path).WithTry("add a line: version: 1")
	}
	if v != 1 {
		return errs.Configf("%s has version %d", path, v).
			WithWhy("this bam understands version 1").
			WithTry("upgrade bam, or check the file")
	}
	return nil
}
