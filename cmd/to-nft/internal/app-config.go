package internal

import (
	"time"

	"github.com/H-BF/sgroups/internal/config"
)

/*// config-sample.yaml

netns: NetworkNS #is optional; def-val = ""
graceful-shutdown: 10s
logger:
  level: INFO

extapi:
  svc:
    def-daial-duration: 10s
    sgroups:
	  dial-duration: 3s #override default-connect-tmo
      address: tcp://127.0.0.1:9006
	  check-changed: 20s #optional
*/

const (
	//AppLoggerLevel log level [optional]
	AppLoggerLevel config.ValueT[string] = "logger/level"
	//AppGracefulShutdown [optional]
	AppGracefulShutdown config.ValueT[time.Duration] = "graceful-schutdown"
	// NetNS network namespace
	NetNS config.ValueT[string] = "netns"

	//ServicesDefDialDuration default dial duraton to conect a service [optional]
	ServicesDefDialDuration config.ValueT[time.Duration] = "extapi/svc/def-daial-duration"

	//SGroupsAddress service address [mandatory]
	SGroupsAddress config.ValueT[string] = "extapi/svc/sgroups/address"
	//SGroupsDialDuration sgroups service dial duration [optional]
	SGroupsDialDuration config.ValueT[time.Duration] = "extapi/svc/sgroups/dial-duration"
	//SGroupsSyncStatusInterval interval(duration) backend sync-status check [mandatory]
	SGroupsSyncStatusInterval config.ValueT[time.Duration] = "extapi/svc/sgroups/check-sync-status"
)
