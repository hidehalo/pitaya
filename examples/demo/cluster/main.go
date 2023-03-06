package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"github.com/topfreegames/pitaya/v2"
	"github.com/topfreegames/pitaya/v2/acceptor"
	"github.com/topfreegames/pitaya/v2/cluster"
	"github.com/topfreegames/pitaya/v2/component"
	"github.com/topfreegames/pitaya/v2/config"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/simulate"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/storage"
	"github.com/topfreegames/pitaya/v2/examples/demo/cluster/services/world"
	"github.com/topfreegames/pitaya/v2/groups"
	"github.com/topfreegames/pitaya/v2/route"
	"github.com/topfreegames/pitaya/v2/serialize/jsonpb"
)

var app pitaya.Pitaya

func configureBackend() {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // use default Addr
		Password: "",               // no password set
		DB:       0,                // use default DB
	})

	// room := services.NewRoom(app)
	// app.Register(room,
	// 	component.WithName("room"),
	// 	component.WithNameFunc(strings.ToLower),
	// )

	app.Register(world.NewWorld(app, storage.NewRedisStreamManager(redisClient), redisClient),
		component.WithName("world"),
		component.WithNameFunc(strings.ToLower),
	)

	simulator := simulate.NewSimulateComponent(storage.NewRedisStreamManager(redisClient), app, redisClient)
	app.Register(simulator,
		component.WithName("simulator"),
		component.WithNameFunc(strings.ToLower),
	)

	// app.RegisterRemote(room,
	// 	component.WithName("room"),
	// 	component.WithNameFunc(strings.ToLower),
	// )
}

func configureFrontend() {
	app.Register(services.NewConnector(app),
		component.WithName("connector"),
		component.WithNameFunc(strings.ToLower),
	)

	app.RegisterRemote(services.NewConnectorRemote(app),
		component.WithName("connectorremote"),
		component.WithNameFunc(strings.ToLower),
	)

	err := app.AddRoute("room", func(
		ctx context.Context,
		route *route.Route,
		payload []byte,
		servers map[string]*cluster.Server,
	) (*cluster.Server, error) {
		// will return the first server
		for k := range servers {
			return servers[k], nil
		}
		return nil, nil
	})

	if err != nil {
		fmt.Printf("error adding route %s\n", err.Error())
	}

	err = app.SetDictionary(map[string]uint16{
		"connector.getsessiondata": 1,
		"connector.setsessiondata": 2,
		"room.room.getsessiondata": 3,
		"onMessage":                4,
		"onMembers":                5,
	})

	if err != nil {
		fmt.Printf("error setting route dictionary %s\n", err.Error())
	}
}

func main() {
	var err error
	cfg := viper.New()
	cfg.AddConfigPath(".")
	cfg.SetConfigName("config")
	cfg.SetConfigType("toml")
	err = cfg.ReadInConfig()
	if err != nil {
		panic(err)
	}
	serverConfig := config.NewConfig(cfg)
	isCluster := serverConfig.GetBool("pitaya.cluster.enabled")
	var svMode pitaya.ServerMode
	if isCluster {
		svMode = pitaya.Cluster
	} else {
		svMode = pitaya.Standalone
	}
	svType := serverConfig.GetString("pitaya.cluster.node.type")
	var isFrontend bool
	listens := make([]map[string]interface{}, 0)
	err = serverConfig.UnmarshalKey("pitaya.conn.listens", &listens)
	if err != nil {
		panic(err)
	}
	if len(listens) > 0 {
		isFrontend = true
	}
	builder := pitaya.NewBuilderWithConfigs(isFrontend, svType, svMode, map[string]string{}, serverConfig)
	for _, listen := range listens {
		if scheme, ok := listen["scheme"]; ok {
			if port, ok := listen["port"]; ok {
				fmt.Println("listen", scheme, port)
				if scheme == "tcp" {
					builder.AddAcceptor(acceptor.NewTCPAcceptor(fmt.Sprintf(":%d", port)))
				} else if scheme == "ws" {
					builder.AddAcceptor(acceptor.NewWSAcceptor(fmt.Sprintf(":%d", port)))
				}
			}
		}
	}
	// builder.Groups = groups.NewMemoryGroupService(*config.NewDefaultMemoryGroupConfig())
	etcdGrpConf := config.NewEtcdGroupServiceConfig(serverConfig)
	builder.Groups, err = groups.NewEtcdGroupService(*etcdGrpConf, nil)
	if err != nil {
		log.Fatal(err)
	}
	// builder.Serializer = protobuf.NewSerializer()
	builder.Serializer = jsonpb.NewSerializer()
	app = builder.Build()
	if !isFrontend {
		configureBackend()
	} else {
		configureFrontend()
		http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("web"))))
		go http.ListenAndServe(fmt.Sprintf(":%d", serverConfig.GetInt("app.chat.web.port")), nil)
	}
	pitaya.SetTimerPrecision(5 * time.Millisecond)
	defer app.Shutdown()
	app.Start()
}
