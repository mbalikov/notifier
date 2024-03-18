package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func StartInputs(Context *_context) {
	inputs := &Context.Config.Inputs
	for ii := range len(inputs.Sockets) {
		go inputSocket(Context, &inputs.Sockets[ii])
	}
	for ii := range len(inputs.Folders) {
		go inputFolder(Context, &inputs.Folders[ii])
	}
	for ii := range len(inputs.Pipes) {
		go inputPipe(Context, &inputs.Pipes[ii])
	}
	for ii := range len(inputs.Http) {
		go inputHttp(Context, &inputs.Http[ii])
	}
}

func inputSocket(Context *_context, in *_inSocketConfig) {
	log.Printf("Starting SOCKET input: %s:%s", in.Type, in.Address)
	Context.ActiveInputs.Add(1)
	defer Context.ActiveInputs.Done()

	if in.Type == "udp" {
		log.Fatalf("udp sockets are not supported")
		return
	}

	if in.Type == "unix" {
		// Remove the socket file if it already exists
		os.Remove(in.Address)
		defer os.Remove(in.Address)
	}

	timeout := time.Duration(in.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = Context.InputTimeout
	}

	// Setup Listener
	unix_timeout := unix.Timeval{Sec: int64(timeout / time.Second), Usec: int64(timeout % time.Second)}
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			if err := c.Control(func(fd uintptr) {
				opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
				if opErr != nil {
					return
				}
				opErr = unix.SetsockoptTimeval(int(fd), unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix_timeout)
			}); err != nil {
				return err
			}
			return opErr
		},
	}

	// Start listener
	l, err := lc.Listen(context.Background(), in.Type, in.Address)
	if err != nil {
		log.Fatalf("INPUT-SOCKET: Error listening on socket %s: %v", in.Address, err)
	}
	defer l.Close()

	// Wait for stop
	go func() {
		<-Context.StopChan
		l.Close()
	}()

	// Process incoming connection
	for {
		if IsStopping(Context) {
			break
		}

		conn, err := l.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				break
			}
			log.Printf("INPUT-SOCKET: Error accepting connection on socket %s: %v", in.Address, err)
			continue
		}

		// New client
		go func(c net.Conn) {
			defer c.Close()

			c.SetReadDeadline(time.Now().Add(timeout))
			buf, err := io.ReadAll(conn)
			if err != nil {
				log.Printf("INPUT-SOCKET: Error reading from connection on socket %s: %v", in.Address, err)
				return
			}

			message := string(buf)
			message = strings.TrimSpace(message)
			if message != "" {
				Context.Messages <- message
			}
		}(conn)
	}
	log.Printf("Stopping SOCKET input: %s:%s", in.Type, in.Address)
}

func inputFolder(Context *_context, in *_inFolderConfig) {
	log.Printf("Starting FOLDER input: %s", in.Path)
	Context.ActiveInputs.Add(1)
	defer Context.ActiveInputs.Done()

	scan_t := time.Duration(in.ScanTime) * time.Millisecond
	for {
		if IsStopping(Context) {
			log.Printf("Stopping FOLDER input: %s", in.Path)
			return
		}
		err := filepath.WalkDir(in.Path,
			func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil // skip directories
				}

				fileName := d.Name()
				if in.FilePrefix != "" &&
					!strings.HasPrefix(fileName, in.FilePrefix) {
					return nil
				}
				if in.FileSuffix != "" &&
					!strings.HasSuffix(fileName, in.FileSuffix) {
					return nil
				}

				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if err := os.Remove(path); err != nil {
					return err
				}

				message := string(content)
				message = strings.TrimSpace(message)
				if message != "" {
					Context.Messages <- message
				}
				return nil
			})
		if err != nil {
			log.Fatalf("INPUT-FOLDER: Error scanning %s : %v\n", in.Path, err)
		}

		// Interruptable sleep
		select {
		case <-time.After(scan_t):
		case <-Context.StopChan:
		}
	}
}

func inputPipe(Context *_context, in *_inPipeConfig) {
	log.Printf("Starting PIPE input on %s", in.Path)
	Context.ActiveInputs.Add(1)
	defer Context.ActiveInputs.Done()

	// Create pipe
	err := syscall.Mkfifo(in.Path, 0666)
	if err != nil && !os.IsExist(err) {
		log.Fatalf("INPUT-PIPE: Error creating named pipe %s : %s", in.Path, err)
		return
	}

	// Process messages
	for {
		fd, err := syscall.Open(in.Path,
			syscall.O_RDONLY|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0666)
		if err != nil {
			log.Fatalf("INPUT-PIPE: Error opening named pipe %s : %s", in.Path, err)
			return
		}
		defer syscall.Close(fd)

		// prepare select
		rfdset := &syscall.FdSet{}
		fdset_ZERO(rfdset)

		buf := make([]byte, 1024*1024)
		for {
			if IsStopping(Context) {
				log.Printf("Stopping PIPE input: %s", in.Path)
				return
			}

			// check File Description status
			fdset_Set(rfdset, fd)
			timeout := syscall.Timeval{Sec: 0, Usec: 100_000}

			n, err_s := syscall.Select(fd+1, rfdset, nil, nil, &timeout)
			if err_s != nil {
				if err_s == syscall.EAGAIN || err_s == syscall.EINTR {
					continue
				}
				log.Fatalf("INPUT-PIPE: Error polling the pipe %s : %s", in.Path, err_s)
				break
			}
			if n == 0 || !fdset_IsSet(rfdset, fd) {
				continue // select timeout
			}

			// ready to read
			n, err_r := syscall.Read(fd, buf)
			if err_r != nil {
				if err_r == syscall.EAGAIN || err_r == syscall.EINTR {
					continue
				}
				log.Fatalf("INPUT-PIPE: Error reading from pipe %s : %s", in.Path, err_r)
				break
			}
			if n == 0 { // EOF
				break
			}

			message := string(buf[:n])
			message = strings.TrimSpace(message)
			if message != "" {
				Context.Messages <- message
			}
		}
		syscall.Close(fd)
	}
}

func inputHttp(Context *_context, in *_inHttpConfig) {
	log.Printf("Starting HTTP input: %s", in.Address)
	Context.ActiveInputs.Add(1)
	defer Context.ActiveInputs.Done()

	timeout := time.Duration(in.Timeout) * time.Millisecond
	if timeout == 0 {
		timeout = Context.InputTimeout
	}

	// Setup Listener
	unix_timeout := unix.Timeval{Sec: int64(timeout / time.Second), Usec: int64(timeout % time.Second)}
	lc := net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			if err := c.Control(func(fd uintptr) {
				opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
				if opErr != nil {
					return
				}
				opErr = unix.SetsockoptTimeval(int(fd), unix.SOL_SOCKET, unix.SO_RCVTIMEO, &unix_timeout)
			}); err != nil {
				return err
			}
			return opErr
		},
	}
	// Start listener
	l, err := lc.Listen(context.Background(), "tcp", in.Address)
	if err != nil {
		log.Fatalf("INPUT-HTTP: Error listening on socket %s: %v", in.Address, err)
	}
	defer l.Close()

	// Setup HTTP server
	http_handler := func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Print("INPUT-HTTP: error reading request body: ", err)
			fmt.Fprintf(w, "Error reading request body")
			return
		}
		defer r.Body.Close()
		w.WriteHeader(http.StatusOK)

		message := string(body)
		message = strings.TrimSpace(message)
		if message != "" {
			Context.Messages <- message
		}
	}

	http_srv := &http.Server{
		Addr:         in.Address,
		Handler:      http.HandlerFunc(http_handler),
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
		IdleTimeout:  timeout,
	}

	// Start HTTP server
	go func() {
		if err := http_srv.Serve(l); err != nil {
			if err != http.ErrServerClosed {
				log.Fatalf("INPUT-HTTP: srror starting server %s : %s\n",
					in.Address, err)
			}
		}
	}()

	// Wait for stop signal
	<-Context.StopChan
	log.Printf("Stopping HTTP input: %s", in.Address)

	// Create a context with a timeout for the server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := http_srv.Shutdown(ctx); err != nil {
		log.Printf("INPUT-HTTP: shutdown of %s failed: %s", in.Address, err)
	}
}
