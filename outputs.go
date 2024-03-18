package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os/exec"
	"time"
)

func outputEmail(msg_ctx *MessageContext, out *_outEmailConfig) {
	ActiveWorkers.Increment()
	defer ActiveWorkers.Decrement()

	smtpHost := replaceJSONPathTags(msg_ctx, out.SmtpHost, &out.tags.SmtpHost)
	smtpPort := replaceJSONPathTags(msg_ctx, out.SmtpPort, &out.tags.SmtpPort)
	smtpUser := replaceJSONPathTags(msg_ctx, out.SmtpUser, &out.tags.SmtpUser)
	smtpPass := replaceJSONPathTags(msg_ctx, out.SmtpPass, &out.tags.SmtpPass)

	from := replaceJSONPathTags(msg_ctx, out.From, &out.tags.From)
	to := replaceJSONPathTags(msg_ctx, out.To, &out.tags.To)

	subject := replaceJSONPathTags(msg_ctx, out.Subject, &out.tags.Subject)
	body := replaceJSONPathTags(msg_ctx, out.Body, &out.tags.Body)

	emailBody := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		from, to, subject, body)

	timeout := time.Duration(out.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = msg_ctx.Context.OutputTimeout
	}

	err := sendEmail(smtpHost, smtpPort, smtpUser, smtpPass,
		from, []string{to}, []byte(emailBody), timeout)
	if err != nil {
		log.Printf("OUTPUT-EMAIL: error sending email to %s:%s : %s",
			smtpHost, smtpPort, err)
	}
}

func outputSocket(msg_ctx *MessageContext, out *_outSocketConfig) {
	ActiveWorkers.Increment()
	defer ActiveWorkers.Decrement()

	sockType := replaceJSONPathTags(msg_ctx, out.Type, &out.tags.Type)
	sockAddr := replaceJSONPathTags(msg_ctx, out.Address, &out.tags.Address)

	timeout := time.Duration(out.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = msg_ctx.Context.OutputTimeout
	}

	conn, err := net.DialTimeout(sockType, sockAddr, timeout)
	if err != nil {
		log.Printf("OUTPUT-SOCKET: failed to connect to socket %s:%s : %v\n",
			sockType, sockAddr, err)
		return
	}
	defer conn.Close()

	// The message to send
	message := replaceJSONPathTags(msg_ctx, out.Message, &out.tags.Message)

	// Send the message
	conn.SetWriteDeadline(time.Now().Add(timeout))
	_, err = conn.Write([]byte(message))
	if err != nil {
		log.Printf("OUTPUT-SOCKET: failed to write to socket %s:%s : %v\n",
			out.Type, out.Address, err)
		return
	}
}

func outputHttp(msg_ctx *MessageContext, out *_outHttpPostConfig) {
	ActiveWorkers.Increment()
	defer ActiveWorkers.Decrement()

	url := replaceJSONPathTags(msg_ctx, out.Url, &out.tags.Url)
	method := replaceJSONPathTags(msg_ctx, out.Method, &out.tags.Method)
	body := replaceJSONPathTags(msg_ctx, out.Body, &out.tags.Body)

	req, err := http.NewRequest(method, url, bytes.NewBuffer([]byte(body)))
	if err != nil {
		log.Printf("OUTPUT-HTTP: failed to create http request to %s : %s", url, err)
		return
	}

	if out.tags.HeadersKeys == nil { // initialize tags cache for Headers
		out.tags.HeadersKeys = make([]*[]string, len(out.Headers))
		out.tags.HeadersVals = make([]*[]string, len(out.Headers))
	}

	for ii := range len(out.Headers) {
		for h_k, h_v := range out.Headers[ii] {
			h_k = replaceJSONPathTags(msg_ctx, h_k, &out.tags.HeadersKeys[ii])
			h_v = replaceJSONPathTags(msg_ctx, h_v, &out.tags.HeadersVals[ii])
			req.Header.Set(h_k, h_v)
		}
	}

	timeout := time.Duration(out.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = msg_ctx.Context.OutputTimeout
	}
	http_client := &http.Client{
		Timeout: timeout,
	}
	resp, err := http_client.Do(req)
	if err != nil {
		log.Printf("OUTPUT-HTTP: failed to send HTTP request to %s err: %s", url, err)
		return
	}

	// ignore reponse
	resp.Body.Close()
}

func execCommand(msg_ctx *MessageContext, exec_conf *_execCommandConfig) {
	ActiveWorkers.Increment()
	defer ActiveWorkers.Decrement()

	if exec_conf.tags.Args == nil { // initialize tags cache for Args
		exec_conf.tags.Args = make([]*[]string, len(exec_conf.Args))
	}

	cmd := replaceJSONPathTags(msg_ctx, exec_conf.Cmd, &exec_conf.tags.Cmd)
	args := make([]string, len(exec_conf.Args))
	for i := range len(exec_conf.Args) {
		args[i] = replaceJSONPathTags(msg_ctx, exec_conf.Args[i], &exec_conf.tags.Args[i])
	}

	timeout := time.Duration(exec_conf.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = msg_ctx.Context.OutputTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	run := exec.CommandContext(ctx, cmd, args...)
	output, err := run.CombinedOutput()
	if err != nil {
		log.Printf("EXEC: Failed to execute command \"%s\" : %s : %s\n",
			cmd, err, string(output))
	}
	cancel()
}
