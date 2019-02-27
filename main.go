package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/minio/minio-go"
	"gopkg.in/mattes/go-expand-tilde.v1"
)

var accessKey string
var secretKey string
var endpoint string
var bucket string
var prefix string
var monitorPath string

func init() {
	flag.StringVar(&accessKey, "access-key", "", "Access key")
	flag.StringVar(&secretKey, "secret-key", "", "Secret key")
	flag.StringVar(&endpoint, "endpoint", "nyc3.digitaloceanspaces.com", "Endpoint URL")
	flag.StringVar(&bucket, "bucket", "veo", "Bucket (or Space) name")
	flag.StringVar(&prefix, "prefix", "", "Key prefix")
	flag.StringVar(&monitorPath, "monitor-path", "~/Desktop", "Path to monitor")
	flag.Parse()

	if len(accessKey) == 0 {
		log.Fatal("access-key cannot be blank")
	}
	if len(secretKey) == 0 {
		log.Fatal("secret-key cannot be blank")
	}
	if len(endpoint) == 0 {
		log.Fatal("endpoint cannot be blank")
	}
	if len(bucket) == 0 {
		log.Fatal("bucket cannot be blank")
	}
	if len(monitorPath) == 0 {
		log.Fatal("monitor-path cannot be blank")
	}
	var err error
	if monitorPath, err = tilde.Expand(monitorPath); err != nil {
		log.Fatal(err)
	}
}

func listObjects(client *minio.Client) {
	doneCh := make(chan struct{})
	defer close(doneCh)
	for o := range client.ListObjectsV2(bucket, prefix, true, doneCh) {
		if o.Err != nil {
			fmt.Println(o.Err)
			return
		}
		fmt.Println(o)
	}
}

func main() {
	client, err := minio.New(endpoint, accessKey, secretKey, true)
	if err != nil {
		log.Fatal(err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := watcher.Close(); err != nil {
			log.Println(err)
		}
	}()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				switch {
				case event.Op&fsnotify.Write == fsnotify.Write:
					log.Println("modified file:", event.Name)
				case event.Op&fsnotify.Create == fsnotify.Create:
					f := filepath.Join(prefix, filepath.Base(event.Name))
					_, err := client.FPutObject(bucket, f, event.Name, minio.PutObjectOptions{})
					if err != nil {
						log.Printf("error writing to storage: %s", err.Error())
						continue
					}
					u, err := client.PresignedGetObject(bucket, f, 20*time.Minute, url.Values{})
					if err != nil {
						log.Printf("error getting share url: %s", err.Error())
						continue
					}
					fmt.Println("Share URL:", u.String())
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	go listObjects(client)

	err = watcher.Add(monitorPath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}
