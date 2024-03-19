# JSON-RPC NOTIFICATION DISPATCHER

Flexiable service to receive JSON-RPC notifications and based on the method to send messages or to execute commands.

**Use-case:** I'm running a large number of docker containers and I needed a way to receive notifications if there is a crash or errors. With this module I have a single place to configure notification dispatching and I can use simple shell commands - just mount the unix socket or named pipe to the docker container.

## Inputs:
* Sockets UNIX and TCP
* Named PIPEs
* Scanning folders for json files
* HTTP servers
 
## Outputs:
* Sockets: UNIX or TCP
* HTTP POST
* Email
* Execute commands
 

## JSONPath
Each output field supports templating with JSONPath format.  For example:  
Message:
```
{
    "method": "alert-trap",
    "params": {
        "key": "alert.high",
        "val": "IT'S THE END OF THE WORLD"
    }
}
```

config.yaml:
```
[...]
methods:
    alert-trap:
        email:
            - smtp-host: localhost
              smtp-port: 25
              #smtp-user: notifier@example.com
              #smtp-pass: 123456
              from: notifier@example.com
              to: CHANNEL-EMAIL@WORKSPACE.slack.com
              subject: 'Notification: {{$.key}}'
              body: '{{$.val}}'
        socket:
            - type: unix
              address: /run/zabbix/sender.sock
              message: '- "{{$.key}}" "{{$.val}}"'
        exec:
            - cmd: /usr/bin/zabbix_sender
              args:
                - '-c'
                - '/etc/zabbix/zabbix_agent2.conf'
                - '-k' 
                - '"{{$.key}}"' 
                - "-o" 
                - '"{{$.value}}"'
[...]
```

## Example
Check config.yaml for detailed examples.


## Concurrency
1. Each input and output handler is run in separate goroutine
2. On signal SIGINT(2) or SIGTERM(15) will stop gracefully by flushing the message queue.
3. On signal SIGHUP(1) will reload the config file with minimum downtime.
4. All input sockets are with SO_REUSEPORT, so several processes could be started to process in parallel.


## Build
```
go mod tidy
go build
mv notifier /usr/local/bin/
mkdir /etc/notifier/
mv config.yaml /etc/notifier/
systemctl enable ./systemd/notifier.service
```

