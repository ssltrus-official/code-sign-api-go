package codesignapi

import (
	"os"
	"runtime/debug"
)

var defaultDebug = func() bool {
	names := []string{"SSLTRUS_CODESIGN_DEBUG", "SSLTRUS_DEBUG", "DEBUG"}
	for _, name := range names {
		value, ok := os.LookupEnv(name)
		if ok {
			switch value {
			case "1", "t", "true", "yes", "y", "on":
				return true
			default:
				return false
			}
		}
	}
	return false
}()

const modulePath = "github.com/ssltrus-official/code-sign-api-go"

var sdkVersion = func() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if ok {
		if buildInfo.Main.Path == modulePath {
			return buildInfo.Main.Version
		}
		for _, dependency := range buildInfo.Deps {
			if dependency.Path == modulePath {
				return dependency.Version
			}
		}
	}
	return "devel"
}()
