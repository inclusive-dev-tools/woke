package ignore

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"github.com/rs/zerolog/log"

	"github.com/get-woke/woke/pkg/util"
)

// Ignore is a gitignore-style object to ignore files/directories
type Ignore struct {
	matcher gitignore.Matcher
}

// IgnoreFactory defines dependency functions so it's much easier to mock during testing
type IgnoreFactory struct {
	filepathAbs func(path string) (string, error)
	osGetwd     func() (string, error)
}

var defaultIgnoreFiles = []string{
	".gitignore",
	".ignore",
	".wokeignore",
	".git/info/exclude",
}

// Default dependency function definitions
func NewIgnoreFactory() (iFactory *IgnoreFactory) {
	return &IgnoreFactory{
		filepathAbs: filepath.Abs,
		osGetwd:     os.Getwd,
	}
}

// Given a workingDir (example: /root/proj/subDir/curDir)
// and given a gitRoot (example: /root/proj)
// it will return []string{"subDir", "curDir"}.
// This is the domain (path from git root) that ignore rules will apply to.
func getDomainFromWorkingDir(workingDir, gitRoot string) []string {
	// if working directory does not end with a slash, add it
	if !strings.HasSuffix(gitRoot, string(os.PathSeparator)) {
		gitRoot += string(os.PathSeparator)
	}

	res := strings.SplitN(workingDir, gitRoot, 2)
	if len(res) > 1 {
		x := util.FilterEmptyStrings(strings.Split(res[1], string(os.PathSeparator)))
		return x
	}
	return []string{}
}

func (iFactory *IgnoreFactory) GetRootGitDir(workingDir string) (filesystem billy.Filesystem, err error) {
	dir, err := iFactory.filepathAbs(workingDir)
	if err != nil {
		return nil, err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return osfs.New(dir), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			log.Debug().Msg("Could Not Find Root Git Folder")
			return osfs.New(workingDir), nil
		}
		dir = parent
	}
}

// NewIgnore produces an Ignore object, with compiled lines from defaultIgnoreFiles
// which you can match files against
func (iFactory *IgnoreFactory) NewIgnore(filesystem billy.Filesystem, lines []string) (ignore *Ignore, err error) {
	start := time.Now()
	defer func() {
		log.Debug().
			TimeDiff("durationMS", time.Now(), start).
			Msg("finished compiling ignores")
	}()

	var cwd string
	if cwd, err = iFactory.osGetwd(); err != nil {
		return nil, err
	}

	var ps []gitignore.Pattern
	if ps, err = gitignore.ReadPatterns(filesystem, nil, defaultIgnoreFiles); err != nil {
		return nil, err
	}

	// get domain for git ignore rules supplied from the lines argument
	domain := getDomainFromWorkingDir(cwd, filesystem.Root())
	for _, line := range lines {
		pattern := gitignore.ParsePattern(line, domain)
		ps = append(ps, pattern)
	}

	ignore = &Ignore{
		matcher: gitignore.NewMatcher(ps),
	}

	return
}

// Match returns true if the provided file matches any of the defined ignores
func (i *Ignore) Match(f string, isDir bool) bool {
	parts := util.FilterEmptyStrings(strings.Split(f, string(os.PathSeparator)))
	return i.matcher.Match(parts, isDir)
}
