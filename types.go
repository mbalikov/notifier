package main

import (
	"sync"
	"time"
)

type JsonRpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Id      interface{} `json:"id"`
}

type MessageContext struct {
	JsonRpc        JsonRpcRequest
	JSONPath_Cache map[string]string // per message cache of resolved JSONPath tags

	Context *_context
}

// ========================================================
// INPUTS
// ========================================================
type _inSocketConfig struct {
	Type    string `mapstructure:"type"`
	Address string `mapstructure:"address"`
	Timeout uint32 `mapstructure:"timeout"`
}

type _inFolderConfig struct {
	Path       string `mapstructure:"path"`
	FilePrefix string `mapstructure:"file-prefix"`
	FileSuffix string `mapstructure:"file-suffix"`
	ScanTime   uint32 `mapstructure:"scan-time"`
	Timeout    uint32 `mapstructure:"timeout"`
}

type _inPipeConfig struct {
	Path    string `mapstructure:"path"`
	Timeout uint32 `mapstructure:"timeout"`
}

type _inHttpConfig struct {
	Address string `mapstructure:"address"`
	Timeout uint32 `mapstructure:"timeout"`
}

type _inputConfig struct {
	Sockets []_inSocketConfig `mapstructure:"sockets"`
	Folders []_inFolderConfig `mapstructure:"folders"`
	Pipes   []_inPipeConfig   `mapstructure:"pipes"`
	Http    []_inHttpConfig   `mapstructure:"http"`
}

// ========================================================
// OUTPUTS
// ========================================================
type _outEmailConfig struct {
	SmtpHost string `mapstructure:"smtp-host"`
	SmtpPort string `mapstructure:"smtp-port"`
	SmtpUser string `mapstructure:"smtp-user"`
	SmtpPass string `mapstructure:"smtp-pass"`
	From     string `mapstructure:"from"`
	To       string `mapstructure:"to"`
	Subject  string `mapstructure:"subject"`
	Body     string `mapstructure:"body"`
	Timeout  uint32 `mapstructure:"timeout"`

	// Cache parsed TAGs from parsed strings
	tags struct {
		SmtpHost *[]string
		SmtpPort *[]string
		SmtpUser *[]string
		SmtpPass *[]string
		From     *[]string
		To       *[]string
		Subject  *[]string
		Body     *[]string
	}
}

type _outSocketConfig struct {
	Type    string `mapstructure:"type"`
	Address string `mapstructure:"address"`
	Message string `mapstructure:"message"`
	Timeout uint32 `mapstructure:"timeout"`

	tags struct {
		Type    *[]string
		Address *[]string
		Message *[]string
	}
}

type _outHttpPostConfig struct {
	Url     string              `mapstructure:"url"`
	Method  string              `mapstructure:"method"`
	Headers []map[string]string `mapstructure:"headers"`
	Body    string              `mapstructure:"body"`
	Timeout uint32              `mapstructure:"timeout"`

	tags struct {
		Url         *[]string
		Method      *[]string
		HeadersKeys []*[]string
		HeadersVals []*[]string
		Body        *[]string
	}
}

type _execCommandConfig struct {
	Cmd     string   `mapstructure:"cmd"`
	Args    []string `mapstructure:"args"`
	Timeout uint32   `mapstructure:"timeout"`

	tags struct {
		Cmd  *[]string
		Args []*[]string
	}
}

type _methodConfig struct {
	Email  []_outEmailConfig    `mapstructure:"email"`
	Http   []_outHttpPostConfig `mapstructure:"http"`
	Socket []_outSocketConfig   `mapstructure:"socket"`
	Exec   []_execCommandConfig `mapstructure:"exec"`
}

type _context struct {
	Config struct {
		Inputs  _inputConfig             `mapstructure:"inputs"`
		Methods map[string]_methodConfig `mapstructure:"methods"`

		QueueSize uint32 `mapstructure:"queue_size"`
		Workers   uint32 `mapstructure:"workers"`
	}

	// Default timeouts
	InputTimeout  time.Duration
	OutputTimeout time.Duration
	ExecTimeout   time.Duration

	Messages     chan string
	ActiveInputs sync.WaitGroup
	StopChan     chan bool
}
