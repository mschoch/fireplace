package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/go-kit/log"
	_ "github.com/lib/pq"
)

func main() {

	var (
		metaDir  = flag.String("meta.dir", "meta", "Directory to store metadata")
		dataDir  = flag.String("data.dir", "data", "Directory to store data")
		appsDir  = flag.String("apps.dir", "apps", "Directory to store apps")
		tsnetDir = flag.String("tsnet.dir", "tsnet", "Directory to store tsnet state")
		logts    = flag.Bool("logts", false, "Log tsnet activity")
	)
	flag.Parse()

	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.With(logger, "ts", log.DefaultTimestampUTC)
		logger = log.With(logger, "caller", log.DefaultCaller)
	}

	// ensure that the directories we require exist
	err := ensureDirectoriesExist(*metaDir, *dataDir, *tsnetDir, *appsDir)
	if err != nil {
		logger.Log("msg", "error ensuring required directory exists", "err", err)
		os.Exit(1)
	}

	dataStore := NewFileSystemDataStore(*dataDir)
	metaStore := NewFileSystemMetaStore(*metaDir, logger)
	err = metaStore.Init()
	if err != nil {
		logger.Log("msg", "error initializing meta store", "err", err)
		os.Exit(1)
	}

	var s Service
	{
		s = newService(logger, dataStore, metaStore, *appsDir, *tsnetDir, *logts)
		s = newLoggingMiddleware(logger)(s)
	}

	errs := make(chan error)
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errs <- fmt.Errorf("%s", <-c)
	}()

	err = autoStartApps(*appsDir, s, logger)
	if err != nil {
		logger.Log("msg", "error autostarting apps", "err", err.Error())
		os.Exit(1)
	}

	logger.Log("exit", <-errs)
}

func autoStartApps(appDir string, s Service, logger log.Logger) error {
	err := filepath.Walk(appDir, func(path string, info os.FileInfo, err error) error {
		if err != nil && os.IsNotExist(err) {
			// if dir doesn't exist, ignore
			return nil
		} else if err != nil {
			return err
		}
		if !info.IsDir() && info.Name() == "app.json" {
			appBytes, err2 := os.ReadFile(path)
			if err2 != nil {
				return err2
			}
			var app Application
			err = json.Unmarshal(appBytes, &app)
			if err != nil {
				logger.Log("msg", "error parsing todo app", "err", err.Error())
				os.Exit(1)
			}
			if app.AutoStart {
				logger.Log("msg", "launching app", "name", app.Name)
				err = s.LaunchApplication(&app)
				if err != nil {
					logger.Log("msg", "error launching application", "err", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking meta dir: %w", err)
	}

	return nil
}

func ensureDirectoriesExist(paths ...string) error {
	for _, path := range paths {
		err := os.MkdirAll(path, 0700)
		if err != nil {
			return fmt.Errorf("mkdirall %q error: %w", path, err)
		}
	}
	return nil
}
