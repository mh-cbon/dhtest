package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
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

		pbk, err := ed25519.PbkFromHex("77ff84905a91936367c01360803104f92432fcd904a43511876df5cdf3e7e548")
		if err != nil {
			panic(err)
		}
		target := "6c1a73a41dbd4412ac81dac66d95b5c33ec83e30"
		for {
			fmt.Printf("Performing lookup request for %v\n", target)
			lookupErr := d.LookupStores(target, nil)
			if lookupErr != nil {
				log.Println(lookupErr)
				continue
			}
			val, err := d.MGetAll(target, pbk, 0, "salt")
			if err != nil {
				fmt.Println("errs", err)
				id, _ := dht.HexToBytes(target)
				ids := shitfID(id, 10)
				var wg sync.WaitGroup
				wg.Add(len(ids))
				for _, id := range ids {
					todoTarget := fmt.Sprintf("%x", id)
					fmt.Printf("Performing lookup request for %v\n", todoTarget)
					go func() {
						defer wg.Done()
						lookupErr := d.LookupStores(todoTarget, nil)
						if lookupErr != nil {
							log.Println(lookupErr)
						}
						addrs, _ := d.ClosestStores(todoTarget, 8)
						x := []*net.UDPAddr{}
						for _, a := range addrs {
							x = append(x, a.GetAddr())
						}
						v, err := d.MGetAll(todoTarget, pbk, 0, "salt", x...)
						if err != nil {
							log.Println(err)
						}
						if v != "" {
							val = v
						}
					}()
				}
				wg.Wait()
			}
			if val != "" {
				doQuery(val)
			}
			<-time.After(time.Second * 10)
		}
		return nil
	}

	ln, err := utp.Listen("127.0.0.1:8002")
	if err != nil {
		panic(err)
	}

	sock := ln.(*utp.Socket)
	if err := dht.New(dht.Opts.WithSocket(sock), dht.DefaultOps()).Serve(ready); err != nil {
		if err != nil {
			log.Fatal(err)
		}
	}

}

func doQuery(addr string) {

	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return utp.Dial(addr)
			},
		},
	}
	resp, err := client.Get(addr)
	if err != nil {
		log.Println(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
	}

	fmt.Println(string(body))
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
