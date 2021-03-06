package main

import (
	"./types"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/joho/godotenv"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"time"
)

var (
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func pprintResp(r io.Reader) error {
	b, err := ioutil.ReadAll(r)
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

// Retrieve an access token from redit given several credentials.
func getToken() (*types.AccessToken, error) {
	CLIENT_ID := os.Getenv("DEBBIE_CLIENT_ID")
	CLIENT_SECRET := os.Getenv("DEBBIE_CLIENT_SECRET")
	AUTH_URL := "https://www.reddit.com/api/v1/access_token"

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
	req.Header.Set("User-Agent", "Debbie/0.1 by laodicean")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	token := &types.AccessToken{Created: time.Now().Unix()}
	err = json.NewDecoder(resp.Body).Decode(token)
	if err != nil {
		return nil, err
	}

	return token, nil
}

func getSubmissions(t *types.AccessToken, subreddit string, page string, limit int, after string) ([]types.Submission, error) {
	data := url.Values{
		"limit": {strconv.Itoa(limit)},
	}
	if after != "" {
		data.Add("after", after)
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://oauth.reddit.com/r/%s/%s", subreddit, page)+"?"+data.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "bearer "+t.Access_token)
	req.Header.Set("User-Agent", "Debbie/0.1 by laodicean")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type Response struct {
		Data struct {
			Children []struct {
				Data *types.Submission
			}
		}
	}

	r := &Response{}
	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	subs := make([]types.Submission, 0, limit)

	for _, child := range r.Data.Children {
		subs = append(subs, *child.Data)
	}

	return subs, nil
}

func getComments(t *types.AccessToken, sub *types.Submission) ([]*types.Comment, error) {
	data := url.Values{
		"context": {"0"},
		"sort":    {"old"},
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://oauth.reddit.com/r/%s/comments/%s", sub.Subreddit, sub.Id)+"?"+data.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "bearer "+t.Access_token)
	req.Header.Set("User-Agent", "Debbie/0.1 by laodicean")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	type Response struct {
		Data struct {
			Children []struct {
				Data *types.Comment
			}
		}
	}

	//pprintResp(resp.Body)
	parts := &[2]Response{}

	err = json.NewDecoder(resp.Body).Decode(parts)
	if err != nil {
		// Supress errors when we try to unmarshal a "more comments" child
		if _, ok := err.(*json.UnmarshalTypeError); !ok {
			Warning.Println(err)
		}
	}

	comments := make([]*types.Comment, 0, 500)

	for _, child := range parts[1].Data.Children {
		unpack_comment_replies(&comments, child.Data)
	}

	return comments, nil
}

func unpack_comment_replies(comments *[]*types.Comment, parent *types.Comment) {
	*comments = append(*comments, parent)
	for _, reply := range parent.Replies.Data.Children {
		unpack_comment_replies(comments, reply.Data)
	}
}

func main() {
	//create reader for readstring calls later on
	//reader := bufio.NewReader(os.Stdin)
	Info = log.New(os.Stderr, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	Warning = log.New(os.Stderr, "Warning: ", log.Ldate|log.Ltime|log.Lshortfile)
	Error = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)

	if err := godotenv.Load(); err != nil {
		Error.Println(err)
		panic(err)
	}

	token, err := getToken()
	if err != nil {
		Error.Println(err)
		panic(err)
	}

	// We need to limit ourselves to 60 requests per second
	networkSem := make(types.Semaphore, 60)
	networkSem.P(60)

	ticker := time.NewTicker(time.Second)
	go func() {
		for _ = range ticker.C {
			networkSem.P(1)
		}
	}()

	subs := []types.Submission{}
	after := ""
	pages := 20
	Info.Println("Requesting", pages*100, "hot submissions from /r/all")
	for i := 0; i < pages; i++ {
		networkSem.V(1)
		new_subs, err := getSubmissions(token, "all", "hot", 100, after)
		if err != nil {
			Error.Println(err)
		}

		subs = append(subs, new_subs...)
		after = subs[len(subs)-1].Name
		Info.Printf("%d%%\n", (i+1)*(100/pages))
	}

	poor_comments := make(types.Comments, 0, 100)

	for _, sub := range subs {
		Info.Println(sub.Score, sub.Subreddit, sub.Title)
		networkSem.V(1)

		comments, err := getComments(token, &sub)
		if err != nil {
			Error.Println(err)
		}

		for _, comment := range comments {
			if comment.Score < -100 {
				Info.Println("\n", comment.Score, time.Unix(int64(comment.Created), 0), comment.Subreddit, comment.Name, "\n", comment.Body)
				comment.Submission = sub
				poor_comments = append(poor_comments, *comment)
			}
		}
	}

	sort.Sort(poor_comments)

	for _, pc := range poor_comments {
		fmt.Println("--------------")
		fmt.Printf("%d [%s] %s\n", pc.Score, pc.Submission.Subreddit, pc.Submission.Title)
		fmt.Println("https://np.reddit.com" + pc.Submission.Permalink + pc.Id + "?context=10")
		fmt.Println(pc.Body)
	}

	ticker.Stop()
}
