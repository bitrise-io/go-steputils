package rubyscript

import (
	"github.com/bitrise-io/go-utils/env"
	"path"

	"github.com/bitrise-io/go-utils/command"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
)

// TODO remove
var temporaryFactory = command.NewFactory(env.NewRepository())

// TODO remove
var pathProvider = func(rootPath string, file string) string {
	return path.Join(rootPath, file)
}

// Helper ...
type Helper struct {
	scriptContent string
	tmpDir        string
	gemfilePth    string
}

// New ...
func New(scriptContent string) Helper {
	return Helper{
		scriptContent: scriptContent,
	}
}

func (h *Helper) ensureTmpDir() (string, error) {
	if h.tmpDir != "" {
		return h.tmpDir, nil
	}

	tmpDir, err := pathutil.NormalizedOSTempDirPath("__ruby-script-runner__")
	if err != nil {
		return "", err
	}

	h.tmpDir = tmpDir

	return tmpDir, nil
}

// BundleInstallCommand ...
func (h *Helper) BundleInstallCommand(gemfileContent, gemfileLockContent string) (command.Command, error) {
	tmpDir, err := h.ensureTmpDir()
	if err != nil {
		return nil, err
	}

	gemfilePth := pathProvider(tmpDir, "Gemfile")
	if err := fileutil.WriteStringToFile(gemfilePth, gemfileContent); err != nil {
		return nil, err
	}

	if gemfileLockContent != "" {
		gemfileLockPth := pathProvider(tmpDir, "Gemfile.lock")
		if err := fileutil.WriteStringToFile(gemfileLockPth, gemfileLockContent); err != nil {
			return nil, err
		}
	}

	h.gemfilePth = gemfilePth

	// use '--gemfile=<gemfile>' flag to specify Gemfile path
	// ... In general, bundler will assume that the location of the Gemfile(5) is also the project root,
	// and will look for the Gemfile.lock and vendor/cache relative to it. ...
	// TODO inject
	return temporaryFactory.Create("bundle", []string{"install", "--gemfile=" + gemfilePth}, nil), nil
}

// RunScriptCommand ...
func (h Helper) RunScriptCommand() (command.Command, error) {
	tmpDir, err := h.ensureTmpDir()
	if err != nil {
		return nil, err
	}

	rubyScriptPth := pathProvider(tmpDir, "script.rb")
	if err := fileutil.WriteStringToFile(rubyScriptPth, h.scriptContent); err != nil {
		return nil, err
	}

	var cmd command.Command
	// TODO inject
	if h.gemfilePth != "" {
		opts := &command.Opts{Env: []string{"BUNDLE_GEMFILE=" + h.gemfilePth}}
		cmd = temporaryFactory.Create("bundle", []string{"exec", "ruby", rubyScriptPth}, opts)
	} else {
		cmd = temporaryFactory.Create("ruby", []string{rubyScriptPth}, nil)
	}

	return cmd, nil
}
