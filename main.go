package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/zishang520/engine.io/log"
	"github.com/zishang520/engine.io/utils"
	"github.com/zishang520/engine.io/v2/types"
	"github.com/zishang520/socket.io-go-redis/adapter"
	rtypes "github.com/zishang520/socket.io-go-redis/types"
	"github.com/zishang520/socket.io/v2/socket"
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

	log.DEBUG = true

	rdbClient := rtypes.NewRedisClient(context.Background(), redis.NewClient(&redis.Options{
		Addr:     "redis:6379",
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

		sessions := []string{}
		io.In(socket.Room("room")).FetchSockets()(func(sockets []*socket.RemoteSocket, err error) {
			if err != nil {
				logger.Error("Failed to fetch socketio sockets; error: %v", err)
			}
			for _, sock := range sockets {
				sessions = append(sessions, string(sock.Id()))
			}
		})
		logger.Info("all socketio clients in room: %v", sessions)

		client.On("event", func(datas ...any) {
		})
		client.On("disconnect", func(...any) {
		})
	})
	httpServer.Listen(":3000", nil)

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
