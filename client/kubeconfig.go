package client

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
)

const DEFAULT_KUBECONFIG_PATH = ".kubeconfig"

type KubeconfigPathOption string

// WithSilent will stop the function from printing information to the console.
const WithSilent KubeconfigPathOption = "silent"

// LookupKubeconfigPath checks if a kubeconfig file exists, and returns its path.
//
// It first checks the "STACC_KUBECONFIG" environment variable, followed
// by looking in the current directory.
//
// Use the WithSilent KubeconfigPathOption to stop this function from
// logging helpful information directly to the console.
func LookupKubeconfigPath(options ...KubeconfigPathOption) (string, error) {
	var kubeconfigPath string

	useSilent := false
	for _, option := range options {
		switch option {
		case WithSilent:
			useSilent = true
		default:
			return "", fmt.Errorf("unknown option %q", option)
		}
	}

	var useEnvVar bool
	staccKubeconfigEnv := os.Getenv("STACC_KUBECONFIG")
	if staccKubeconfigEnv != "" {
		useEnvVar = true
		kubeconfigPath = staccKubeconfigEnv
	} else {
		var err error
		kubeconfigPath, err = filepath.Abs(DEFAULT_KUBECONFIG_PATH)
		if err != nil {
			return "", err
		}
	}

	if !useSilent {
		reportFlowRCDeprecated()
	}

	info, err := os.Stat(kubeconfigPath)
	if err != nil {
		if os.IsNotExist(err) && !useSilent {
			if useEnvVar {
				color.Red("\"STACC_KUBECONFIG\" environment variable is set to %q, but this file does not exist", kubeconfigPath)
			} else {
				color.Red("No \".kubeconfig\" file found in current directory %q", filepath.Dir(kubeconfigPath))
				color.Red("Make sure you are in the correct directory, or set the \"STACC_KUBECONFIG\" environment variable to reference the correct file")
			}
		}
		return "", err
	}

	if info.IsDir() {
		if useEnvVar && !useSilent {
			color.Red("\"STACC_KUBECONFIG\" environment variable is set to %q, but this is a directory", kubeconfigPath)
		}
		return "", fmt.Errorf("client: %q is a directory", kubeconfigPath)
	}

	return kubeconfigPath, nil
}

func reportFlowRCDeprecated() {
	flowRCAbsPath, err := filepath.Abs(".flowrc")
	if err != nil {
		color.Red("Something went wrong: %s", err)
		return
	}

	_, err = os.Stat(flowRCAbsPath)
	if err == nil {
		color.Yellow("WARNING: .flowrc is deprecated and should be removed")
	}
}
