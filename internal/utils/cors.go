package utils

import (
	"strings"

	"github.com/dovakiin0/proxy-m3u8/config"
)

func GetCorsDomain() []string {
	corsDomain := config.Env.CorsDomain

	allowOrigins := []string{}
	if corsDomain == "*" {
		allowOrigins = append(allowOrigins, "*")
	} else {
		domains := strings.Split(corsDomain, ",")
		for _, domain := range domains {
			if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
				allowOrigins = append(allowOrigins, strings.TrimSuffix(domain, "/"))
			} else {
				allowOrigins = append(allowOrigins, "http://"+strings.TrimSuffix(domain, "/"))
				allowOrigins = append(allowOrigins, "https://"+strings.TrimSuffix(domain, "/"))
			}
		}
	}

	return allowOrigins
}
