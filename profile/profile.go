package profile

import (
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"strings"

	"github.com/zhangel/go-framework/utils"

	"github.com/zhangel/go-framework/config"
	"github.com/zhangel/go-framework/declare"
	"github.com/zhangel/go-framework/lifecycle"
	"github.com/zhangel/go-framework/log"
)

const (
	profilePrefix            = "pprof"
	flagEnable               = "enable"
	flagAddr                 = "addr"
	flagPort                 = "port"
	flagMutexProfileFraction = "mutex_profile_fraction"
	flagBlockProfileRate     = "block_profile_rate"
)

var profileConfig config.Config

func init() {
	declare.Flags(profilePrefix,
		declare.Flag{Name: flagEnable, DefaultValue: false, Description: "Enable pprof (Must be disabled in production env!)."},
		declare.Flag{Name: flagAddr, DefaultValue: "", Description: "Address of pprof server."},
		declare.Flag{Name: flagPort, DefaultValue: 6060, Description: "Port of pprof server."},
		declare.Flag{Name: flagMutexProfileFraction, DefaultValue: 0, Description: "The fraction of mutex contention events that are reported in the mutex profile."},
		declare.Flag{Name: flagBlockProfileRate, DefaultValue: 0, Description: "the fraction of goroutine blocking events that are reported in the blocking profile."},
	)

	lifecycle.LifeCycle().HookInitialize(func() {
		if !IsProfileEnabled() {
			return
		}

		go func() {
			serverAddr := Config().String(flagAddr)
			if strings.TrimSpace(serverAddr) == "" {
				serverAddr = config.String("server.addr")
				if serverAddr != "" {
					if strings.HasPrefix(serverAddr, "[::]:") {
						serverAddr = serverAddr[strings.LastIndex(serverAddr, ":"):]
					}

					if serverAddr[0] == ':' {
						serverAddr = ""
					} else {
						serverAddr, _, _ = net.SplitHostPort(serverAddr)
					}
				}

				if serverAddr == "" {
					if ip, err := utils.HostIp(); err == nil {
						serverAddr = ip + serverAddr
					}
				}
			}

			runtime.SetMutexProfileFraction(Config().Int(flagMutexProfileFraction))
			runtime.SetBlockProfileRate(Config().Int(flagBlockProfileRate))

			fmt.Printf("PProf listen on http://%s:%d\n", serverAddr, Config().Int(flagPort))
			if err := http.ListenAndServe(fmt.Sprintf("%s:%d", serverAddr, Config().Int(flagPort)), nil); err != nil {
				log.Fatalf("Start pprof server failed, err = %v", err)
			}
		}()
	}, lifecycle.WithName("Enable pprof"))
}

func Config() config.Config {
	if profileConfig == nil {
		profileConfig = config.WithPrefix(profilePrefix)
	}
	return profileConfig
}

func IsProfileEnabled() bool {
	return Config().Bool(flagEnable)
}
