package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"
)

const maxPageSize int = 100

func main() {
	cloudflareAccountIDPtr := flag.String("cf-acct-id", "", "cloudflare account id")
	cloudflareAPIKeyPtr := flag.String("cf-api-key", "", "cloudflare api key")
	flag.Parse()

	if *cloudflareAccountIDPtr == "" || *cloudflareAPIKeyPtr == "" {
		flag.Usage()
		return
	}

	httpCli := http.DefaultClient
	httpCli.Timeout = time.Second * 15

	cloudflareCli := &cloudflareClient{
		httpCli:   httpCli,
		accountID: *cloudflareAccountIDPtr,
		apiKey:    *cloudflareAPIKeyPtr,
	}

	unprotectedImages, err := cloudflareCli.getUnprotectedImages()
	if err != nil {
		log.Fatalln("failed to get images id:", err)
	}

	var wg sync.WaitGroup

	for _, imageID := range unprotectedImages {
		wg.Add(1)

		go func(wg *sync.WaitGroup, id string) {
			defer wg.Done()
			if err := cloudflareCli.secureImage(id); err != nil {
				log.Printf("failed to secure image '%s': %s", id, err)
				return
			}
			log.Printf("successfully secured image '%s'", id)
		}(&wg, imageID)
	}
	wg.Wait()

	// Fetch gain to see if they are still unprotected images left.
	unprotectedImages, err = cloudflareCli.getUnprotectedImages()
	if err != nil {
		log.Fatalln("failed to get images id:", err)
	}

	if len(unprotectedImages) > 0 {
		log.Printf("%d images left unprotected", len(unprotectedImages))
	}

	log.Println("done")
}

type cloudflareClient struct {
	httpCli   *http.Client
	accountID string
	apiKey    string
}

type cloudflareResponse struct {
	Result struct {
		Images []struct {
			ID                string `json:"id"`
			RequireSignedURLs bool   `json:"requireSignedURLs"`
		} `json:"images"`
	} `json:"result"`
	Success bool `json:"success"`
}

// getUnprotectedImages makes a request to cloudflare to list all the images
// and returns the ids of the ones that have required signed url set to false.
// Does not support pagination, but that is not a problem for now.
// https://api.cloudflare.com/#cloudflare-images-list-images
func (c *cloudflareClient) getUnprotectedImages() ([]string, error) {
	u := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/images/v1?page=1&per_page=%d", c.accountID, maxPageSize)

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("could not prepare request: %s", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not send request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var listImagesResp cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&listImagesResp); err != nil {
		return nil, fmt.Errorf("could not decode response: %s", err)
	}

	if !listImagesResp.Success {
		return nil, fmt.Errorf("list images response not successful")
	}

	if len(listImagesResp.Result.Images) == maxPageSize {
		log.Println("there's probably more pages to go through")
	}

	var unprotectedImages []string
	for _, image := range listImagesResp.Result.Images {
		if !image.RequireSignedURLs {
			unprotectedImages = append(unprotectedImages, image.ID)
		}
	}
	return unprotectedImages, nil
}

// secureImage makes a request to cloudflare to update the image to require signed urls.
func (c *cloudflareClient) secureImage(imageID string) error {
	u := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/images/v1/%s", c.accountID, imageID)
	req, err := http.NewRequest(http.MethodPatch, u, nil)
	if err != nil {
		return fmt.Errorf("could not prepare request: %s", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Add("Content-Type", "application/json")

	reqBody := []byte(`{"requireSignedURLs": true}`)
	req.Body = ioutil.NopCloser(bytes.NewReader(reqBody))
	req.ContentLength = int64(len(reqBody))

	resp, err := c.httpCli.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var updateImageResp cloudflareResponse
	if err := json.NewDecoder(resp.Body).Decode(&updateImageResp); err != nil {
		return fmt.Errorf("could not decode response: %s", err)
	}

	if !updateImageResp.Success {
		return fmt.Errorf("update image response not successful")
	}
	return nil
}
