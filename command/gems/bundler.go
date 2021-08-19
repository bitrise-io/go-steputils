package gems

import (
	"fmt"
	"github.com/bitrise-io/go-utils/env"
	"os/exec"

	"github.com/bitrise-io/go-steputils/command/rubycommand"
	"github.com/bitrise-io/go-utils/command"
)

// TODO remove
var temporaryFactory = command.NewFactory(env.NewRepository())

// InstallBundlerCommand returns a command to install a specific bundler version
func InstallBundlerCommand(gemfileLockVersion Version) command.Command {
	args := []string{"install", "bundler", "--force", "--no-document"}
	if gemfileLockVersion.Found {
		args = append(args, []string{"--version", gemfileLockVersion.Version}...)
	}

	return temporaryFactory.Create("gem", args, nil)
}

// BundleInstallCommand returns a command to install a bundle using bundler
func BundleInstallCommand(gemfileLockVersion Version) (command.Command, error) {
	var args []string
	if gemfileLockVersion.Found {
		args = append(args, "_"+gemfileLockVersion.Version+"_")
	}
	args = append(args, []string{"install", "--jobs", "20", "--retry", "5"}...)

	return rubycommand.New("bundle", args...)
}

// BundleExecPrefix returns a slice containing: "bundle [_verson_] exec"
func BundleExecPrefix(bundlerVersion Version) []string {
	bundleExec := []string{"bundle"}
	if bundlerVersion.Found {
		bundleExec = append(bundleExec, fmt.Sprintf("_%s_", bundlerVersion.Version))
	}
	return append(bundleExec, "exec")
}

// RbenvVersionsCommand retruns a command to print used and available ruby versions if rbenv is installed
func RbenvVersionsCommand() command.Command {
	if _, err := exec.LookPath("rbenv"); err != nil {
		return nil
	}

	return temporaryFactory.Create("rbenv", []string{"versions"}, nil)
}
