package main

import (
	"flag"
	"fmt"
	"github.com/asiainfoLDP/datafoundry_coupon/api"
	"github.com/asiainfoLDP/datafoundry_coupon/log"
	"github.com/asiainfoLDP/datafoundry_coupon/models"
	"github.com/asiainfoLDP/datafoundry_coupon/router"
	"github.com/asiainfoLDP/datahub_commons/httputil"
	"net/http"
	"time"
)

const SERVERPORT = 8574

var (
	debug = flag.Bool("debug", false, "debug mode")
	local = flag.Bool("local", false, "running on local")

	logger = log.GetLogger()

	//init a router
	initRouter = router.InitRouter()
)

type Service struct {
	httpPort int
}

func newService(httpPort int) *Service {
	service := &Service{
		httpPort: httpPort,
	}

	return service
}

func main() {

	//new a router
	router.NewRouter(initRouter)

	// init db
	models.InitDB()

	service := newService(SERVERPORT)
	address := fmt.Sprintf(":%d", service.httpPort)
	logger.Debug("address: %v", address)

	logger.Info("Listening http at: %s", address)
	err := http.ListenAndServe(address, httputil.TimeoutHandler(initRouter, 2500*time.Millisecond, ""))
	if err != nil {
		logger.Error("http listen and server err: %v", err)
		return
	}

	return
}

func init() {
	//api.InitMQ()

	flag.Parse()
	log.SetDebug = *debug
	models.SetPlatform = *local

	log.InitLog()

	api.InitGateWay()
}
