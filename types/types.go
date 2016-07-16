package types

type AccessToken struct {
	Access_token  string
	Token_type    string
	Expires_in    int
	Scope         string
	Refresh_token string
	Created       int64 // to be set as the struct is cretaed.
}

type Submission struct {
	Id           string
	Score        int32
	Num_comments int
	Subreddit    string
	Created      float64
}

type Comment struct {
	Subreddit_id string
	Score        int32
	Body         string
	Name         string
	Created      float64
	Replies      struct {
		Data struct {
			Children []struct {
				Data *Comment
			}
		}
	}
}