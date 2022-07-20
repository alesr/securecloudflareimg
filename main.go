package main

import (
	"flag"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/alesr/securecloudflareimage/cloudflareclient"
)

func main() {
	cloudflareAccountIDPtr := flag.String("account-id", "", "cloudflare account id")
	cloudflareAPIKeyPtr := flag.String("api-key", "", "cloudflare api key")
	flag.Parse()

	if *cloudflareAccountIDPtr == "" || *cloudflareAPIKeyPtr == "" {
		flag.Usage()
		return
	}

	httpCli := http.DefaultClient
	httpCli.Timeout = time.Second * 15

	cloudflareCli := cloudflareclient.New(httpCli, *cloudflareAccountIDPtr, *cloudflareAPIKeyPtr)

	unprotectedImages, err := cloudflareCli.GetUnprotectedImages()
	if err != nil {
		log.Fatalln("failed to get unprotected images:", err)
	}

	var wg sync.WaitGroup

	for _, imageID := range unprotectedImages {
		wg.Add(1)

		go func(wg *sync.WaitGroup, id string) {
			defer wg.Done()
			if err := cloudflareCli.SecureImage(id); err != nil {
				log.Printf("failed to secure image '%s': %s", id, err)
				return
			}

			log.Printf("successfully secured image '%s'", id)
		}(&wg, imageID)
	}
	wg.Wait()

	// Fetch gain to see if they are still unprotected images left.
	unprotectedImages, err = cloudflareCli.GetUnprotectedImages()
	if err != nil {
		log.Fatalln("failed to get images id:", err)
	}

	if len(unprotectedImages) > 0 {
		log.Printf("%d images left unprotected", len(unprotectedImages))
	}

	log.Println("done")
}
