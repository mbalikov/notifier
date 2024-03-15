package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type _inSocketConfig struct {
	Type    string `mapstructure:"type"`
	Address string `mapstructure:"address"`
}

type _inFolderConfig struct {
	Path       string `mapstructure:"path"`
	FilePrefix string `mapstructure:"file-prefix"`
	FileSuffix string `mapstructure:"file-suffix"`
}

type _outSocketConfig struct {
	Type    string `mapstructure:"type"`
	Address string `mapstructure:"address"`
	Message string `mapstructure:"message"`
}

type _config struct {
	inputSockets []_inSocketConfig
	inputFolder  _inFolderConfig

	// smtp host to send message
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

	outputSockets []_outSocketConfig

	// external commands to execute
	shellCommands []string
}

var Config = _config{}

func main() {
	var configName string
	flag.StringVar(&configName, "config", "", "yaml config file name without extension")
	flag.Parse()

	if configName == "" {
		log.Fatalf("USAGE: notifier config.yaml")
	}

	initConfig(configName)
	loadConfig()

	messages := make(chan string, 1000)

	for _, socketPath := range Config.inputSockets {
		go listenOnSocket(socketPath, messages)
	}

	if Config.inputFolder.Path != "" {
		go func() {
			for {
				scanInputFolder(messages)
				time.Sleep(time.Second)
			}
		}()
	}

	for {
		msg := <-messages
		msg = strings.TrimSpace(msg)
		handleMessage(msg)
	}
}

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

// ========================================================
// CONFIG
// ========================================================
func loadConfig() {
	// input
	if err := viper.UnmarshalKey("input.sockets", &Config.inputSockets); err != nil {
		log.Fatalf("Missing \"input.sockets\": %s", err)
	}

	if err := viper.UnmarshalKey("input.folder", &Config.inputFolder); err != nil {
		log.Fatalf("Error in \"input.folder\": %s", err)
	}

	// output
	Config.smtpHost = viper.GetString("smtp.host")
	Config.smtpPort = viper.GetString("smtp.port")
	Config.smtpFrom = viper.GetString("smtp.from")
	Config.smtpTo = viper.GetString("smtp.to")
	Config.smtpUser = viper.GetString("smtp.user")
	Config.smtpPass = viper.GetString("smtp.password")
	Config.smtpSubject = viper.GetString("smtp.subject")
	Config.smtpBody = viper.GetString("smtp.body")

	if Config.smtpHost != "" && Config.smtpTo != "" && Config.smtpFrom != "" {
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

	if err := viper.UnmarshalKey("output-sockets", &Config.outputSockets); err != nil {
		log.Fatalf("Failed to load sockets: %s", err)
	}

	Config.shellCommands = viper.GetStringSlice("shell-commands")
}

// ========================================================
// HANDLE INBOUND TRAPS
// ========================================================
func listenOnSocket(in_conf _inSocketConfig, messages chan<- string) {
	if in_conf.Type == "udp" {
		log.Fatalf("udp sockets are not supported")
		return
	}

	if in_conf.Type == "unix" {
		// Remove the socket file if it already exists
		os.Remove(in_conf.Address)
		defer os.Remove(in_conf.Address)
	}

	l, err := net.Listen(in_conf.Type, in_conf.Address)
	if err != nil {
		log.Fatalf("Error listening on socket %s: %v", in_conf.Address, err)
	}
	defer l.Close()

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("Error accepting connection on socket %s: %v", in_conf.Address, err)
			continue
		}

		go func(c net.Conn) {
			defer c.Close()

			msg, err := io.ReadAll(conn)
			if err != nil {
				log.Printf("Error reading from connection on socket %s: %v", in_conf.Address, err)
				return
			}

			messages <- string(msg)
		}(conn)
	}
}

func scanInputFolder(messages chan<- string) {
	err := filepath.WalkDir(Config.inputFolder.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil // skip directories
		}

		fileName := d.Name()

		if Config.inputFolder.FilePrefix != "" &&
			!strings.HasPrefix(fileName, Config.inputFolder.FilePrefix) {
			return nil
		}
		if Config.inputFolder.FileSuffix != "" &&
			!strings.HasSuffix(fileName, Config.inputFolder.FileSuffix) {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.Remove(path); err != nil {
			return err
		}

		messages <- string(content)
		return nil
	})

	if err != nil {
		log.Printf("Error scanning directory: %v\n", err)
	}
}

// ========================================================
// PROCESS OUTGOING TRAPS
// ========================================================
func handleMessage(msg string) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(msg), &data); err != nil {
		fmt.Println("Error decoding JSON:", err)
		return
	}

	// Print received key-value pairs
	for key, value_iface := range data {
		value := value_iface.(string)

		if Config.smtpServer != "" {
			sendEmail(key, value)
		}

		for _, out_conf := range Config.outputSockets {
			sendToSocket(out_conf, key, value)
		}

		for _, cmd := range Config.shellCommands {
			cmd = strings.Replace(cmd, "{{KEY}}", key, -1)
			cmd = strings.Replace(cmd, "{{VALUE}}", value, -1)
			execCommand(cmd)
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

func sendToSocket(out_conf _outSocketConfig, key string, value string) {
	conn, err := net.Dial(out_conf.Type, out_conf.Address)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect to socket: %v\n", err)
		return
	}
	defer conn.Close() // Ensure the connection is closed when finished

	// The message to send
	message := out_conf.Message
	message = strings.Replace(message, "{{KEY}}", key, -1)
	message = strings.Replace(message, "{{VALUE}}", value, -1)

	// Send the message
	_, err = conn.Write([]byte(message))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to socket: %v\n", err)
		return
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
