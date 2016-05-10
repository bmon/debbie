package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

type Access_token struct {
	Access_token  string
	Token_type    string
	Expires_in    int
	Scope         string
	Refresh_token string
	created       int64
}

func get_token() (*Access_token, error) {
	CLIENT_ID := os.Getenv("DEBBIE_CLIENT_ID")
	CLIENT_SECRET := os.Getenv("DEBBIE_CLIENT_SECRET")
	AUTH_URL := "https://www.reddit.com/api/v1/access_token"

	client := &http.Client{}
	data := url.Values{
		"grant_type": {"password"},
		"username":   {os.Getenv("DEBBIE_USERNAME")},
		"password":   {os.Getenv("DEBBIE_PASSWORD")},
	}

	req, err := http.NewRequest("POST", AUTH_URL, bytes.NewBufferString((data.Encode())))
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(CLIENT_ID, CLIENT_SECRET)
	req.Header.Set("User-Agent", "DebbieDownvotes/0.1 by laodicean")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	token := &Access_token{created: time.Now().Unix()}
	err = json.NewDecoder(resp.Body).Decode(token)
	if err != nil {
		return nil, err
	}

	for k, v := range resp.Header {
		fmt.Println(k, v)
	}
	fmt.Printf("\n token: %+v\n", token)

	return token, nil
}

func get_hot(t *Access_token, subreddit string, limit string, count string) error {
	client := &http.Client{}
	data := url.Values{
		"limit": {limit},
		"count": {count},
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://oauth.reddit.com/r/%s/hot", subreddit), bytes.NewBufferString((data.Encode())))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+t.Access_token)
	req.Header.Set("User-Agent", "DebbieDownvotes/0.1 by laodicean")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	err = json.Indent(&buf, b, "", "\t")
	if err != nil {
		return err
	}

	fmt.Println(string(buf.Bytes()))
	return nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	token, err := get_token()
	if err != nil {
		fmt.Println(err)
	}

	err = get_hot(token, "all", "1", "0")
	if err != nil {
		panic(err)
	}
}
