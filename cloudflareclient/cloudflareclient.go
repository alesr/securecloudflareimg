package cloudflareclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

const maxPageSize int = 100

type Client struct {
	httpCli   *http.Client
	accountID string
	apiKey    string
}

func New(httpCli *http.Client, accountID, apiKey string) *Client {
	return &Client{
		httpCli:   httpCli,
		accountID: accountID,
		apiKey:    apiKey,
	}
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
func (c *Client) GetUnprotectedImages() ([]string, error) {
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

// SecureImage makes a request to Cloudflare to update the image to require signed URLs.
func (c *Client) SecureImage(imageID string) error {
	u := fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/images/v1/%s", c.accountID, imageID)
	req, err := http.NewRequest(http.MethodPatch, u, nil)
	if err != nil {
		return fmt.Errorf("could not prepare request: %s", err)
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Add("Content-Type", "application/json")

	reqBody := []byte(`{"requireSignedURLs": true}`)
	req.Body = io.NopCloser(bytes.NewReader(reqBody))
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
