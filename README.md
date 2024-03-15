# NOTIFIER

Small service to listen on unix socket and to send receved message to SMTP or array of shell commands.  

JSON format:
```
{
    "KEY1": "VALUE1",
    "KEY2": "VALUE2"
}
```
Each combination of key:value is processed as separate message. 
Start with:
```
notifier --config config.yaml
```
Or with systemd.

Check config.yaml for use examples.
