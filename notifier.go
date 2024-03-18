package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var ActiveWorkers AtomicCounter

func main() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	var configName string
	flag.StringVar(&configName, "config", "", "yaml config file name without extension")
	flag.Parse()
	if configName == "" {
		fmt.Println("USAGE: notifier --config ./config.yaml")
		os.Exit(-1)
	}

	var Context = &_context{}
	if err := InitConfig(configName, Context); err != nil {
		log.Fatal(err)
	}
	StartInputs(Context)

	stopped := false
	for {
		select {
		case msg := <-Context.Messages:
			if ActiveWorkers.Get() < int64(Context.Config.Workers) {
				handleMessage(Context, msg)
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		case sig := <-signalChan:
			if sig == syscall.SIGINT || sig == syscall.SIGTERM {
				if !stopped {
					log.Print("Received SIGTERM: try to stop gracefully")
					close(Context.StopChan)
					stopped = true
				}
			} else if sig == syscall.SIGHUP {
				log.Print("Received SIGHUP: reload config")
				new_Context := Reload(configName, Context)
				if new_Context == nil {
					continue
				}
				Context = new_Context
				StartInputs(Context)
			}
		default:
			if stopped {
				Context.ActiveInputs.Wait()
				for ActiveWorkers.Get() > 0 {
					time.Sleep(100 * time.Millisecond)
				}
				os.Exit(0)
			}
		}
	}
}

func handleMessage(Context *_context, msg string) {
	msg_ctx := MessageContext{
		Context: Context,
	}

	msg = strings.TrimSpace(msg)
	if err := json.Unmarshal([]byte(msg), &msg_ctx.JsonRpc); err != nil {
		log.Printf("Message: error decoding JSON-RPC: %s | err: %s", msg, err)
		return
	}
	msg_ctx.JSONPath_Cache = make(map[string]string)

	method, ok := Context.Config.Methods[msg_ctx.JsonRpc.Method]
	if !ok {
		method, ok = Context.Config.Methods["default"]
		if !ok {
			log.Printf("Message: cannot handle method %s", msg_ctx.JsonRpc.Method)
			return
		}
	}

	for i := range len(method.Email) {
		go outputEmail(&msg_ctx, &method.Email[i])
	}
	for i := range len(method.Socket) {
		go outputSocket(&msg_ctx, &method.Socket[i])
	}
	for i := range method.Http {
		go outputHttp(&msg_ctx, &method.Http[i])
	}
	for i := range method.Exec {
		go execCommand(&msg_ctx, &method.Exec[i])
	}
}

func Reload(configName string, old_Context *_context) *_context {
	var new_Context = &_context{}
	if err := InitConfig(configName, new_Context); err != nil {
		log.Print(err)
		return nil
	}

	// notify inputs to stop
	close(old_Context.StopChan)
	old_Context.ActiveInputs.Wait()

	close(old_Context.Messages)
	for msg := range old_Context.Messages {
		new_Context.Messages <- msg
	}

	return new_Context
}

func IsStopping(Context *_context) bool {
	select {
	default:
		return false
	case <-Context.StopChan:
		return true
	}
}
