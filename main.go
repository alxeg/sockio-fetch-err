package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/redis/go-redis/v9"
	rtypes "github.com/zishang520/socket.io/adapters/redis/v3"
	"github.com/zishang520/socket.io/adapters/redis/v3/adapter"
	"github.com/zishang520/socket.io/servers/socket/v3"
	"github.com/zishang520/socket.io/v3/pkg/log"
	"github.com/zishang520/socket.io/v3/pkg/types"
	"github.com/zishang520/socket.io/v3/pkg/utils"
)

var logger = log.NewLog("test")

func main() {
	httpServer := types.NewWebServer(nil)

	opts := socket.DefaultServerOptions()
	opts.SetAllowEIO3(true) // required
	opts.SetCors(&types.Cors{
		Origin:      "*",
		Credentials: false,
	})

	log.DEBUG.Store(true)

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "redis:6379"
	}

	rdbClient := rtypes.NewRedisClient(context.Background(), redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Username: "",
		Password: "",
		DB:       0,
	}))

	rdbClient.On("error", func(errors ...any) {
		utils.Log().Error("Error: %v", errors)
	})

	opts.SetAdapter(&adapter.RedisAdapterBuilder{
		Redis: rdbClient,
		Opts:  &adapter.RedisAdapterOptions{},
	})

	io := socket.NewServer(httpServer, opts)
	io.On("connection", func(clients ...any) {
		client := clients[0].(*socket.Socket)

		logger.Info("socketio %s connected", client.Id())

		client.Join("room")

		sessions := getConnectedSockets(io)
		logger.Info("onConnect: all socketio clients in room: %v", sessions)

		client.On("event", func(datas ...any) {
		})
		client.On("disconnect", func(...any) {
			sessions := getConnectedSockets(io)
			logger.Info("onDisconnect: all socketio clients in room: %v", sessions)
		})
	})

	listenPort := os.Getenv("PORT")
	if listenPort == "" {
		listenPort = ":3000"
	}
	httpServer.Listen(listenPort, nil)

	exit := make(chan struct{})
	SignalC := make(chan os.Signal)

	signal.Notify(SignalC, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range SignalC {
			switch s {
			case os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				close(exit)
				return
			}
		}
	}()

	<-exit
	httpServer.Close(nil)
	os.Exit(0)
}

func getConnectedSockets(io *socket.Server) []string {
	sessions := []string{}
	var wg sync.WaitGroup
	wg.Add(1)
	io.In(socket.Room("room")).FetchSockets()(func(sockets []*socket.RemoteSocket, err error) {
		defer wg.Done()
		if err != nil {
			logger.Error("Failed to fetch socketio sockets; error: %v", err)
			return
		}
		for _, sock := range sockets {
			sessions = append(sessions, string(sock.Id()))
		}
	})
	wg.Wait()
	return sessions
}
