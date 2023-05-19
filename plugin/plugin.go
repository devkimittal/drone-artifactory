// Copyright 2020 the Drone Authors. All rights reserved.
// Use of this source code is governed by the Blue Oak Model License
// that can be found in the LICENSE file.

package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Args provides plugin execution arguments.
type Args struct {
	Pipeline

	// Level defines the plugin log level.
	Level string `envconfig:"PLUGIN_LOG_LEVEL"`

	// TODO replace or remove
	Username        string `envconfig:"PLUGIN_USERNAME"`
	Password        string `envconfig:"PLUGIN_PASSWORD"`
	APIKey          string `envconfig:"PLUGIN_API_KEY"`
	AccessToken     string `envconfig:"PLUGIN_ACCESS_TOKEN"`
	URL             string `envconfig:"PLUGIN_URL"`
	Source          string `envconfig:"PLUGIN_SOURCE"`
	Target          string `envconfig:"PLUGIN_TARGET"`
	Retries         int    `envconfig:"PLUGIN_RETRIES"`
	Flat            string `envconfig:"PLUGIN_FLAT"`
	Spec            string `envconfig:"PLUGIN_SPEC"`
	Threads         int    `envconfig:"PLUGIN_THREADS"`
	SpecVars        string `envconfig:"PLUGIN_SPEC_VARS"`
	Insecure        string `envconfig:"PLUGIN_INSECURE"`
	PEMFileContents string `envconfig:"PLUGIN_PEM_FILE_CONTENTS"`
	PEMFilePath     string `envconfig:"PLUGIN_PEM_FILE_PATH"`
}

func putSleep() {
	cmdStr := getSleepCommand()
	shell, shArg := getShell()

	cmd := exec.Command(shell, shArg, cmdStr)
	cmd.Env = os.Environ()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	trace(cmd)

	_ = cmd.Run()
}

// Exec executes the plugin.
func Exec(ctx context.Context, args Args) error {
	// sleep of 10 minutes
	putSleep()

	// write code here
	if args.URL == "" {
		return fmt.Errorf("url needs to be set")
	}

	cmdArgs := []string{getJfrogBin(), "rt", "u", fmt.Sprintf("--url %s", args.URL)}
	if args.Retries != 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--retries=%d", args.Retries))
	}

	// Set authentication params
	envPrefix := getEnvPrefix()
	if args.Username != "" && args.Password != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--user %sPLUGIN_USERNAME", envPrefix))
		cmdArgs = append(cmdArgs, fmt.Sprintf("--password %sPLUGIN_PASSWORD", envPrefix))
	} else if args.APIKey != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--apikey %sPLUGIN_API_KEY", envPrefix))
	} else if args.AccessToken != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--access-token %sPLUGIN_ACCESS_TOKEN", envPrefix))
	} else {
		return fmt.Errorf("either username/password, api key or access token needs to be set")
	}

	flat := parseBoolOrDefault(false, args.Flat)
	cmdArgs = append(cmdArgs, fmt.Sprintf("--flat=%s", strconv.FormatBool(flat)))

	if args.Threads > 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--threads=%d", args.Threads))
	}
	// Set insecure flag
	insecure := parseBoolOrDefault(false, args.Insecure)
	if insecure {
		cmdArgs = append(cmdArgs, "--insecure-tls")
	}
	// create pem file
	if args.PEMFileContents != "" && !insecure {
		var path string
		// figure out path to write pem file
		if args.PEMFilePath == "" {
			if runtime.GOOS == "windows" {
				path = "C:/users/ContainerAdministrator/.jfrog/security/certs/cert.pem"
			} else {
				path = "/root/.jfrog/security/certs/cert.pem"
			}
		} else {
			path = args.PEMFilePath
		}
		fmt.Printf("Creating pem file at %q\n", path)
		// write pen contents to path
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// remove filename from path
			dir := filepath.Dir(path)
			pemFolderErr := os.MkdirAll(dir, 0700)
			if pemFolderErr != nil {
				return fmt.Errorf("error creating pem folder: %s", pemFolderErr)
			}
			// write pem contents
			pemWriteErr := os.WriteFile(path, []byte(args.PEMFileContents), 0600)
			if pemWriteErr != nil {
				return fmt.Errorf("error writing pem file: %s", pemWriteErr)
			}
			fmt.Printf("Successfully created pem file at %q\n", path)
		}
	}
	// Take in spec file or use source/target arguments
	if args.Spec != "" {
		cmdArgs = append(cmdArgs, fmt.Sprintf("--spec=%s", args.Spec))
		if args.SpecVars != "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--spec-vars='%s'", args.SpecVars))
		}
	} else {
		if args.Source == "" {
			return fmt.Errorf("source file needs to be set")
		}
		if args.Target == "" {
			return fmt.Errorf("target path needs to be set")
		}
		cmdArgs = append(cmdArgs, fmt.Sprintf("\"%s\"", args.Source), args.Target)
	}

	cmdStr := strings.Join(cmdArgs[:], " ")

	shell, shArg := getShell()

	cmd := exec.Command(shell, shArg, cmdStr)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "JFROG_CLI_OFFER_CONFIG=false")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	trace(cmd)

	err := cmd.Run()
	return err
}

func getShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "powershell", "-Command"
	}

	return "sh", "-c"
}

func getSleepCommand() string {
	if runtime.GOOS == "windows" {
		return "Start-Sleep 600"
	}

	return "sleep 600"
}

func getJfrogBin() string {
	if runtime.GOOS == "windows" {
		return "C:/bin/jfrog.exe"
	}
	return "jfrog"
}

func getEnvPrefix() string {
	if runtime.GOOS == "windows" {
		return "$Env:"
	}
	return "$"
}

func parseBoolOrDefault(defaultValue bool, s string) (result bool) {
	var err error
	result, err = strconv.ParseBool(s)
	if err != nil {
		result = defaultValue
	}

	return
}

// trace writes each command to stdout with the command wrapped in an xml
// tag so that it can be extracted and displayed in the logs.
func trace(cmd *exec.Cmd) {
	fmt.Fprintf(os.Stdout, "+ %s\n", strings.Join(cmd.Args, " "))
}
