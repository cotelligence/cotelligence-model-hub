package version

import (
	"runtime/debug"
)

type AppInfo struct {
	Time     string `json:"time"`
	Revision string `json:"revision"`
}

var appInfo AppInfo

func GetAppInfo() AppInfo {
	return appInfo
}

func init() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				// get the first 8 characters of the commit hash
				if len(setting.Value) > 8 {
					appInfo.Revision = setting.Value[:8]
				}
			}
			if setting.Key == "vcs.time" {
				appInfo.Time = setting.Value
			}
		}
	}
}
