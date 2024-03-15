package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/viper"
)

type _config struct {
	socketPath string

	// smtp host to send message
	hasSmtp     bool
	smtpHost    string
	smtpPort    string
	smtpUser    string
	smtpPass    string
	smtpFrom    string
	smtpTo      string
	smtpSubject string
	smtpBody    string

	smtpServer string
	smtpAuth   smtp.Auth

	// external commands to execute
	commands []string
}

var Config = _config{}

func initConfig(configName string) {
	if configName != "" {
		viper.SetConfigName(configName) // name of config file (without extension)
	} else {
		viper.SetConfigName("config")
	}

	viper.SetConfigType("yaml") // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath(".")    // optionally look for config in the working directory
	viper.AutomaticEnv()        // read in environment variables that match

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}
}

func loadConfig() {
	Config.socketPath = viper.GetString("socket")
	if Config.socketPath == "" {
		log.Fatalf("Missing is missing \"socket\" file path")
	}

	Config.smtpHost = viper.GetString("smtp.host")
	Config.smtpPort = viper.GetString("smtp.port")
	Config.smtpFrom = viper.GetString("smtp.from")
	Config.smtpTo = viper.GetString("smtp.to")
	Config.smtpUser = viper.GetString("smtp.user")
	Config.smtpPass = viper.GetString("smtp.password")
	Config.smtpSubject = viper.GetString("smtp.subject")
	Config.smtpBody = viper.GetString("smtp.body")

	if Config.smtpHost != "" && Config.smtpTo != "" && Config.smtpFrom != "" {
		Config.hasSmtp = true

		if Config.smtpPort == "" {
			Config.smtpPort = "25"
		}
		Config.smtpServer = Config.smtpHost + ":" + Config.smtpPort

		if Config.smtpUser != "" {
			Config.smtpAuth = smtp.PlainAuth("", Config.smtpUser, Config.smtpPass, Config.smtpHost)
		}

		if Config.smtpSubject == "" {
			Config.smtpSubject = "{{KEY}}"
		}
		if Config.smtpBody == "" {
			Config.smtpBody = "{{VALUE}}"
		}
	}
	Config.commands = viper.GetStringSlice("commands")
}

func main() {
	var configName string
	flag.StringVar(&configName, "config", "", "yaml config file name without extension")
	flag.Parse()

	if configName == "" {
		log.Fatalf("USAGE: notifier config.yaml")
	}

	initConfig(configName)
	loadConfig()

	// Start unix socket
	os.Remove(Config.socketPath)
	listener, err := net.Listen("unix", Config.socketPath)
	if err != nil {
		fmt.Println("Error listening on socket:", err)
		return
	}
	defer listener.Close()

	fmt.Println("Listening on", Config.socketPath)
	for {
		// Accept new connection
		conn, err := listener.Accept()
		if err != nil {
			log.Fatalf("Error accepting connection: %s", err)
			continue
		}

		// Handle connection in a new goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	// Read message from connection
	msg, err := io.ReadAll(conn)
	if err != nil {
		fmt.Println("Error reading message:", err)
		return
	}

	// Unmarshal JSON into a map
	var data map[string]interface{}
	if err := json.Unmarshal(msg, &data); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// Print received key-value pairs
	for key, value := range data {
		fmt.Printf("%s: %v\n", key, value)
		if Config.smtpHost != "" {
			sendEmail(key, value.(string))
		}
		if len(Config.commands) > 0 {
			processCommands(key, value.(string))
		}
	}
}

func sendEmail(key string, value string) {
	body := "From: " + Config.smtpFrom + "\r\n" +
		"To: " + Config.smtpTo + "\r\n" +
		"Subject: " + Config.smtpSubject + "\r\n" +
		"\r\n" +
		Config.smtpBody
	body = strings.Replace(body, "{{KEY}}", key, -1)
	body = strings.Replace(body, "{{VALUE}}", value, -1)

	err := smtp.SendMail(
		Config.smtpServer, Config.smtpAuth,
		Config.smtpFrom, []string{Config.smtpTo},
		[]byte(body))
	if err != nil {
		fmt.Println("Error sending email:", err)
		return
	}
}

func processCommands(key string, value string) {
	commands := viper.GetStringSlice("commands")

	for _, cmd := range commands {
		cmd = strings.Replace(cmd, "{{KEY}}", key, -1)
		cmd = strings.Replace(cmd, "{{VALUE}}", value, -1)
		execCommand(cmd)
	}
}

func execCommand(cmd string) {
	// Using 'sh' for Unix-based systems; adjust if necessary, e.g., 'cmd' for Windows
	_, err := exec.Command("sh", "-c", cmd).CombinedOutput()
	if err != nil {
		fmt.Printf("Failed to execute command '%s': %s\n", cmd, err)
		return
	}
}
