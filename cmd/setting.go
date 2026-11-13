package cmd

import (
	"fmt"
	"log"

	"x-ui/config"
	"x-ui/internal/database"
	"x-ui/internal/logger"
	"x-ui/internal/util/crypto"
	"x-ui/internal/web/service"

	"github.com/spf13/cobra"
)

var settingCmd = &cobra.Command{
	Use:   "setting",
	Short: "Manage panel settings",
	Run: func(cmd *cobra.Command, args []string) {
		// Do nothing
	},
}

var (
	port          int
	username      string
	password      string
	webBasePath   string
	listenIP      string
	getListen     bool
	webCertFile   string
	webKeyFile    string
	tgbottoken    string
	tgbotchatid   string
	enabletgbot   bool
	tgbotRuntime  string
	reset         bool
	show          bool
	getCert       bool
	remove_secret bool
	factoryReset  bool
)

func init() {
	rootCmd.AddCommand(settingCmd)

	settingCmd.Flags().BoolVar(&reset, "reset", false, "Reset all settings")
	settingCmd.Flags().BoolVar(&factoryReset, "factory-reset", false, "Factory reset: clear inbounds, clients, staff, and restore default settings & admin user")
	settingCmd.Flags().BoolVar(&show, "show", false, "Display current settings")
	settingCmd.Flags().BoolVar(&remove_secret, "remove_secret", false, "Remove secret key")
	settingCmd.Flags().IntVar(&port, "port", 0, "Set panel port number")
	settingCmd.Flags().StringVar(&username, "username", "", "Set login username")
	settingCmd.Flags().StringVar(&password, "password", "", "Set login password")
	settingCmd.Flags().StringVar(&webBasePath, "webBasePath", "", "Set base path for Panel")
	settingCmd.Flags().StringVar(&listenIP, "listenIP", "", "set panel listenIP IP")
	settingCmd.Flags().BoolVar(&getListen, "getListen", false, "Display current panel listenIP IP")
	settingCmd.Flags().BoolVar(&getCert, "getCert", false, "Display current certificate settings")
	settingCmd.Flags().StringVar(&webCertFile, "webCert", "", "Set path to public key file for panel")
	settingCmd.Flags().StringVar(&webKeyFile, "webCertKey", "", "Set path to private key file for panel")
	settingCmd.Flags().StringVar(&tgbottoken, "tgbottoken", "", "Set token for Telegram bot")
	settingCmd.Flags().StringVar(&tgbotRuntime, "tgbotRuntime", "", "Set cron time for Telegram bot notifications")
	settingCmd.Flags().StringVar(&tgbotchatid, "tgbotchatid", "", "Set chat ID for Telegram bot notifications")
	settingCmd.Flags().BoolVar(&enabletgbot, "enabletgbot", false, "Enable notifications via Telegram bot")

	settingCmd.Run = func(cmd *cobra.Command, args []string) {
		if factoryReset {
			FactoryReset()
		} else if reset {
			ResetSetting()
		} else {
			UpdateSetting(port, username, password, webBasePath, listenIP)
		}
		if show {
			ShowSetting(show)
		}
		if getListen {
			GetListenIP(getListen)
		}
		if getCert {
			GetCertificate(getCert)
		}
		if (tgbottoken != "") || (tgbotchatid != "") || (tgbotRuntime != "") {
			UpdateTgbotSetting(tgbottoken, tgbotchatid, tgbotRuntime)
		}
		if remove_secret {
			RemoveSecret()
		}
		if enabletgbot {
			UpdateTgbotEnableSts(enabletgbot)
		}
	}
}

func FactoryReset() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		return
	}

	settingService := service.SettingService{}
	err = settingService.FactoryReset()
	if err != nil {
		fmt.Println("Failed to perform factory reset:", err)
		return
	}

	serverService := service.ServerService{}
	_ = serverService.RestartXrayService()

	fmt.Println("Panel has been successfully reset to factory settings.")
	fmt.Println("Default login credentials:")
	fmt.Println("  Username: admin")
	fmt.Println("  Password: admin")
	fmt.Println("  Port: 2053")
	fmt.Println("  WebBasePath: /")
}

func ResetSetting() {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Failed to initialize database:", err)
		return
	}

	settingService := service.SettingService{}
	err = settingService.ResetSettings()
	if err != nil {
		fmt.Println("Failed to reset settings:", err)
	} else {
		fmt.Println("Settings successfully reset.")
	}
}

func ShowSetting(show bool) {
	if show {
		settingService := service.SettingService{}
		port, err := settingService.GetPort()
		if err != nil {
			fmt.Println("get current port failed, error info:", err)
		}

		webBasePath, err := settingService.GetBasePath()
		if err != nil {
			fmt.Println("get webBasePath failed, error info:", err)
		}

		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		userService := service.UserService{}
		userModel, err := userService.GetFirstUser()
		if err != nil {
			fmt.Println("get current user info failed, error info:", err)
		}

		if userModel.Username == "" || userModel.Password == "" {
			fmt.Println("current username or password is empty")
		}

		fmt.Println("current panel settings as follows:")
		if certFile == "" || keyFile == "" {
			fmt.Println("Warning: Panel is not secure with SSL")
		} else {
			fmt.Println("Panel is secure with SSL")
		}

		hasDefaultCredential := func() bool {
			return userModel.Username == "admin" && crypto.CheckPasswordHash(userModel.Password, "admin")
		}()

		fmt.Println("hasDefaultCredential:", hasDefaultCredential)
		fmt.Println("port:", port)
		fmt.Println("webBasePath:", webBasePath)
	}
}

func UpdateTgbotEnableSts(status bool) {
	settingService := service.SettingService{}
	currentTgSts, err := settingService.GetTgbotEnabled()
	if err != nil {
		fmt.Println(err)
		return
	}
	logger.Infof("current enabletgbot status[%v],need update to status[%v]", currentTgSts, status)
	if currentTgSts != status {
		err := settingService.SetTgbotEnabled(status)
		if err != nil {
			fmt.Println(err)
			return
		} else {
			logger.Infof("SetTgbotEnabled[%v] success", status)
		}
	}
}

func UpdateTgbotSetting(tgBotToken string, tgBotChatid string, tgBotRuntime string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Error initializing database:", err)
		return
	}

	settingService := service.SettingService{}

	if tgBotToken != "" {
		err := settingService.SetTgBotToken(tgBotToken)
		if err != nil {
			fmt.Printf("Error setting Telegram bot token: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot token.")
	}

	if tgBotRuntime != "" {
		err := settingService.SetTgbotRuntime(tgBotRuntime)
		if err != nil {
			fmt.Printf("Error setting Telegram bot runtime: %v\n", err)
			return
		}
		logger.Infof("Successfully updated Telegram bot runtime to [%s].", tgBotRuntime)
	}

	if tgBotChatid != "" {
		err := settingService.SetTgBotChatId(tgBotChatid)
		if err != nil {
			fmt.Printf("Error setting Telegram bot chat ID: %v\n", err)
			return
		}
		logger.Info("Successfully updated Telegram bot chat ID.")
	}
}

func UpdateSetting(port int, username string, password string, webBasePath string, listenIP string) {
	err := database.InitDB(config.GetDBPath())
	if err != nil {
		fmt.Println("Database initialization failed:", err)
		return
	}

	settingService := service.SettingService{}
	userService := service.UserService{}

	if port > 0 {
		err := settingService.SetPort(port)
		if err != nil {
			fmt.Println("Failed to set port:", err)
		} else {
			fmt.Printf("Port set successfully: %v\n", port)
		}
	}

	if username != "" || password != "" {
		err := userService.UpdateFirstUser(username, password)
		if err != nil {
			fmt.Println("Failed to update username and password:", err)
		} else {
			fmt.Println("Username and password updated successfully")
		}
	}

	if webBasePath != "" {
		err := settingService.SetBasePath(webBasePath)
		if err != nil {
			fmt.Println("Failed to set base URI path:", err)
		} else {
			fmt.Println("Base URI path set successfully")
		}
	}

	if listenIP != "" {
		err := settingService.SetListen(listenIP)
		if err != nil {
			fmt.Println("Failed to set listen IP:", err)
		} else {
			fmt.Printf("listen %v set successfully", listenIP)
		}
	}
}

func GetCertificate(getCert bool) {
	if getCert {
		settingService := service.SettingService{}
		certFile, err := settingService.GetCertFile()
		if err != nil {
			fmt.Println("get cert file failed, error info:", err)
		}
		keyFile, err := settingService.GetKeyFile()
		if err != nil {
			fmt.Println("get key file failed, error info:", err)
		}

		fmt.Println("cert:", certFile)
		fmt.Println("key:", keyFile)
	}
}

func GetListenIP(getListen bool) {
	if getListen {

		settingService := service.SettingService{}
		ListenIP, err := settingService.GetListen()
		if err != nil {
			log.Printf("Failed to retrieve listen IP: %v", err)
			return
		}

		fmt.Println("listenIP:", ListenIP)
	}
}

func RemoveSecret() {
	userService := service.UserService{}

	secretExists, err := userService.CheckSecretExistence()
	if err != nil {
		fmt.Println("Error checking secret existence:", err)
		return
	}

	if !secretExists {
		fmt.Println("No secret exists to remove.")
		return
	}

	err = userService.RemoveUserSecret()
	if err != nil {
		fmt.Println("Error removing secret:", err)
		return
	}

	settingService := service.SettingService{}
	err = settingService.SetSecretStatus(false)
	if err != nil {
		fmt.Println("Error updating secret status:", err)
		return
	}

	fmt.Println("Secret removed successfully.")
}
