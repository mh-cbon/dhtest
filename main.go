package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/anacrolix/utp"
	"github.com/mh-cbon/dht/bootstrap"
	"github.com/mh-cbon/dht/dht"
	"github.com/mh-cbon/dht/ed25519"
	"github.com/mh-cbon/dht/logger"
)

func main() {
	var v bool
	flag.BoolVar(&v, "vv", false, "verbose mode")
	flag.Parse()

	ready := func(d *dht.DHT) error {
		if v {
			d.AddLogger(logger.Text(log.Printf))
		}
		fmt.Println("Running bootstrap...")
		publicIP, err := d.BootstrapAuto(nil, bootstrap.Public)
		for {
			if err == nil {
				break
			}
			fmt.Println("Boostrap err retying...", err)
			publicIP, err = d.BootstrapAuto(nil, bootstrap.Public)
		}
		fmt.Printf("public IP bootstrap %v:%v\n", publicIP.IP, publicIP.Port)

		selfID := []byte(d.ID())
		fmt.Printf("your node id %x\n", selfID)

		fmt.Println("Boostrap done...")

		var bep44testPvk = "e06d3183d14159228433ed599221b80bd0a5ce8352e4bdf0262f76786ef1c74db7e7a9fea2c0eb269d61e3b38e450a22e754941ac78479d6c54e1faf6037881d"
		pvk, _, err := ed25519.PvkFromHex(bep44testPvk)
		if err != nil {
			return err
		}
		put, err := dht.PutFromPvk(fmt.Sprintf("%v:%v", publicIP.IP, publicIP.Port), "salt", pvk, 0, 0)
		if err != nil {
			return err
		}

		for {
			fmt.Printf("Performing lookup request for %v\n", put.Target)
			lookupErr := d.LookupStores(put.Target, nil)
			if lookupErr != nil {
				log.Println(lookupErr)
				continue
			}
			err := d.MPutAll(put)
			if err != nil {
				fmt.Println("errs", err)
				id, _ := dht.HexToBytes(put.Target)
				ids := shitfID(id, 10)
				var wg sync.WaitGroup
				for _, id := range ids {
					todoTarget := fmt.Sprintf("%x", id)
					fmt.Printf("Performing lookup request for %v\n", todoTarget)
					go func() {
						defer wg.Done()
						lookupErr := d.LookupStores(todoTarget, nil)
						if lookupErr != nil {
							log.Println(lookupErr)
						}
						err := d.MPutAll(put)
						if err != nil {
							log.Println(err)
						}
					}()
				}
				wg.Wait()
			}
			<-time.After(time.Second * 10)
		}
		return nil
	}

	ln, err := utp.Listen("0.0.0.0:8000")
	if err != nil {
		panic(err)
	}
	cwd, _ := os.Getwd()
	srv := &http.Server{
		Handler: http.FileServer(http.Dir(cwd)),
	}
	go srv.Serve(ln)

	sock := ln.(*utp.Socket)
	if err := dht.New(dht.Opts.WithSocket(sock), dht.DefaultOps()).Serve(ready); err != nil {
		if err != nil {
			log.Fatal(err)
		}
	}
}

var shift = []byte{0x00, 0x00, 0x00, 0x0C, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F, 0x0F}

func shitfID(id []byte, n int) [][]byte {
	ret := [][]byte{id}
	for i := 1; i < n; i++ {
		src := id
		if i > 0 {
			src = ret[i-1]
		}
		newTarget := make([]byte, len(src))
		for i := range src {
			if src[i] == 255 {
				newTarget[i] = 0
			} else {
				newTarget[i] = src[i] + 1
			}
		}
		ret = append(ret, newTarget)
	}
	return ret
}
