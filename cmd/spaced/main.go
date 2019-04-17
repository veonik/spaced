package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
)

type duration time.Duration

func (d *duration) UnmarshalText(text []byte) error {
	dur, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = duration(dur)
	return nil
}

var config struct {
	S3 s3Config `toml:"s3"`
	URL urlShortenerConfig `toml:"url"`
	FS fswatchConfig `toml:"fswatch"`

	ShareTTL duration `toml:"share_ttl"`

	LogFile string
}

func init() {
	configFile := ""

	flag.Usage = func() {
		fmt.Println("Usage: spaced [options]")
		fmt.Println()
		fmt.Println("Command spaced automatically uploads screenshots and puts a shareable URL in")
		fmt.Println("the system clipboard. The screenshots are uploaded to an S3-compatible bucket,")
		fmt.Println("and eokvin is used as a URL shortening service.")
		fmt.Println()
		fmt.Println("spaced works with an S3-compatible storage, such as DigitalOcean Spaces.")
		fmt.Println()
		fmt.Println("See: https://github.com/veonik/eokvin")
		fmt.Println()
		fmt.Println("Options:")
		flag.PrintDefaults()
	}
	flag.StringVar(&config.S3.AccessKey, "access-key", "", "Access key (required)")
	flag.StringVar(&config.S3.SecretKey, "secret-key", "", "Secret key (required)")
	flag.StringVar(&config.S3.Endpoint, "endpoint", "", "Endpoint URL (required)")
	flag.StringVar(&config.S3.Bucket, "bucket", "", "Bucket name (required)")
	flag.StringVar(&config.S3.Prefix, "prefix", "", "Key prefix")
	flag.StringVar(&config.FS.MonitorPath, "monitor-path", "~/Desktop", "Path to monitor")
	flag.StringVar(&config.LogFile, "log-file", "", "Log file path. If blank, logs print to stdout")
	flag.StringVar(&configFile, "config-file", "config.toml", "Config file path")

	var eokvinToken string
	var eokvinEndpoint string
	flag.StringVar(&eokvinToken, "token", "", "Secret token for eokvin service")
	flag.StringVar(&eokvinEndpoint, "eokvin", "", "URL shortener service endpoint")
	flag.Parse()

	if config.LogFile != "" {
		f, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.FileMode(0644))
		if err != nil {
			log.Fatal("unable to open spaced.log for writing:", err.Error())
		}
		log.SetOutput(f)
	}

	if configFile != "" {
		if _, err := toml.DecodeFile(configFile, &config); err != nil {
			log.Fatal("unable to decode config file:", err.Error())
		}
	}
	if eokvinToken != "" {
		config.URL.Options["token"] = eokvinToken
	}
	if eokvinEndpoint != "" {
		config.URL.Options["endpoint"] = eokvinEndpoint
	}
}

func main() {
	stop := make(chan struct{})
	sigrec := make(chan os.Signal)
	signal.Notify(sigrec, os.Kill, os.Interrupt)

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		worker(stop)
		wg.Done()
	}()

	select {
	case <-stop:
		wg.Wait()
		os.Exit(0)
	case <-sigrec:
		close(stop)
	}
}

func worker(stop chan struct{}) {
	s3, err := config.S3.GetService()
	if err != nil {
		log.Fatal(err)
	}
	c, err := config.URL.GetService()
	if err != nil {
		log.Fatal(err)
	}
	watcher, err := config.FS.GetService()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			log.Println(err)
		}
	}()
	for {
		select {
		case <-stop:
			return

		case event, ok := <-watcher.Events:
			if !ok {
				continue
			}
			switch {
			case event.Op&fsnotify.Create == fsnotify.Create:
				f := filepath.Base(event.Name)
				if strings.HasPrefix(f, ".") {
					//log.Println("new file created, not uploading:", f)
					continue
				} else if !strings.HasSuffix(f, ".png") {
					//log.Println("non png created, not uploading:", f)
					continue
				}
				_, err := s3.FPutObject(f, event.Name)
				if err != nil {
					log.Println("error writing to storage:", err.Error())
					continue
				}
				u, err := s3.PresignedGetObject(f, time.Duration(config.ShareTTL), url.Values{})
				if err != nil {
					log.Println("error getting public aws url:", err.Error())
					continue
				}
				log.Println("AWS URL:", u.String())
				su, err := c.NewShortURL(u, time.Duration(config.ShareTTL))
				if err != nil {
					log.Println("error getting short share url:", err.Error())
					continue
				}
				log.Printf("Share URL: %s (valid until %s)\n",
					su.String(),
					time.Now().Add(time.Duration(config.ShareTTL)).Format("Jan 02 15:04 MST"))
				cmd := exec.Command("pbcopy")
				p, err := cmd.StdinPipe()
				if err != nil {
					log.Println("error opening clipboard:", err.Error())
					continue
				}
				if err := cmd.Start(); err != nil {
					log.Println("error running clipboard command:", err.Error())
					continue
				}
				if _, err := fmt.Fprintf(p, "%s", su.String()); err != nil {
					log.Println("error writing to clipboard:", err.Error())
				}
				if err := p.Close(); err != nil {
					log.Println("error closing clipboard:", err.Error())
				}
				if err := cmd.Wait(); err != nil {
					log.Println("clipboard command exited with error:", err.Error())
					continue
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				close(stop)
				return
			}
			log.Println("error:", err)
		}
	}
}
