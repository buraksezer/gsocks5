package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"log"

	"github.com/hashicorp/logutils"
)

const usage = `gsocks5 -- Secure SOCKS5 proxy server

Usage:
   gsocks5 [command] -c [config-file-path]

Commands:
   -help,   -h  Prints this message.
   -version -v  Prints version.
   -debug   -d  Enables debug mode.
   -config  -c  Configuration file path. It is /etc/gsocks5.json by default.

The Go runtime version %s
Report bugs to https://github.com/purak/gsocks5/issues`

const (
	version           = "0.1"
	defaultConfigPath = "/etc/gsocks5.json"
)

var (
	path        string
	showHelp    bool
	showVersion bool
	debug       bool
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	// Parse command line parameters
	f := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	f.SetOutput(ioutil.Discard)
	f.BoolVar(&showHelp, "h", false, "")
	f.BoolVar(&showHelp, "help", false, "")
	f.BoolVar(&showVersion, "version", false, "")
	f.BoolVar(&showVersion, "v", false, "")
	f.BoolVar(&debug, "d", false, "")
	f.BoolVar(&debug, "debug", false, "")
	f.StringVar(&path, "config", defaultConfigPath, "")
	f.StringVar(&path, "c", defaultConfigPath, "")

	if err := f.Parse(os.Args[1:]); err != nil {
		log.Fatalf("[ERR] Failed to parse flags: %s", err)
	}

	if showHelp {
		msg := fmt.Sprintf(usage, runtime.Version())
		fmt.Println(msg)
		return
	} else if showVersion {
		fmt.Println("gsocks5", version)
		return
	}
	cfg, err := newConfig(path)
	if err != nil {
		log.Fatalf("[ERR] Failed to load configuration: %s", err)
	}

	filter := &logutils.LevelFilter{
		Levels: []logutils.LogLevel{"DEBUG", "WARN", "ERR", "INF"},
		Writer: os.Stderr,
	}
	if debug || cfg.Debug {
		filter.MinLevel = logutils.LogLevel("DEBUG")
	} else {
		filter.MinLevel = logutils.LogLevel("WARN")
	}
	log.SetOutput(filter)

	switch {
	case cfg.Role == roleClient:
		log.Print("[INF] gsocks5: Running as client")
	case cfg.Role == roleServer:
		log.Print("[INF] gsocks5: Running as server")
	}

	srv := newServer(cfg)
	// Handle SIGINT and SIGTERM.
	signal.Notify(srv.signal, syscall.SIGINT, syscall.SIGTERM)

	if err = srv.run(); err != nil {
		log.Fatalf("[ERR] gsocks5: failed to serve %s", err)
	}
	log.Print("[INF] Goodbye!")
}
