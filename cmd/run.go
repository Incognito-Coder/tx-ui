package cmd

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"x-ui/config"
	"x-ui/internal/database"
	"x-ui/internal/logger"
	"x-ui/internal/sub"
	"x-ui/internal/web"
	"x-ui/internal/web/global"

	"github.com/joho/godotenv"
	"github.com/op/go-logging"
	"github.com/spf13/cobra"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the web panel",
	Run: func(cmd *cobra.Command, args []string) {
		RunWebServer()
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func RunWebServer() {
	log.Printf("Starting %v %v", config.GetName(), config.GetVersion())

	switch config.GetLogLevel() {
	case config.Debug:
		logger.InitLogger(logging.DEBUG)
	case config.Info:
		logger.InitLogger(logging.INFO)
	case config.Notice:
		logger.InitLogger(logging.NOTICE)
	case config.Warn:
		logger.InitLogger(logging.WARNING)
	case config.Error:
		logger.InitLogger(logging.ERROR)
	default:
		log.Fatalf("Unknown log level: %v", config.GetLogLevel())
	}

	godotenv.Load()

	err := database.InitDB(config.GetDBPath())
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}

	var server *web.Server
	server = web.NewServer()
	global.SetWebServer(server)
	err = server.Start()
	if err != nil {
		log.Fatalf("Error starting web server: %v", err)
		return
	}

	var subServer *sub.Server
	subServer = sub.NewServer()
	global.SetSubServer(subServer)
	err = subServer.Start()
	if err != nil {
		log.Fatalf("Error starting sub server: %v", err)
		return
	}

	sigCh := make(chan os.Signal, 1)
	// Trap shutdown signals
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, os.Interrupt)

	restartServers := func() {
		err := server.Stop()
		if err != nil {
			logger.Debug("Error stopping web server:", err)
		}
		err = subServer.Stop()
		if err != nil {
			logger.Debug("Error stopping sub server:", err)
		}

		server = web.NewServer()
		global.SetWebServer(server)
		err = server.Start()
		if err != nil {
			log.Fatalf("Error restarting web server: %v", err)
			return
		}
		log.Println("Web server restarted successfully.")

		subServer = sub.NewServer()
		global.SetSubServer(subServer)
		err = subServer.Start()
		if err != nil {
			log.Fatalf("Error restarting sub server: %v", err)
			return
		}
		log.Println("Sub server restarted successfully.")
	}

	for {
		select {
		case sig := <-sigCh:
			switch sig {
			case syscall.SIGHUP:
				logger.Info("Received SIGHUP signal. Restarting servers...")
				restartServers()
			default:
				server.Stop()
				subServer.Stop()
				log.Println("Shutting down servers.")
				return
			}
		case <-global.RestartChan:
			logger.Info("Received panel restart request. Restarting servers...")
			restartServers()
		}
	}
}
