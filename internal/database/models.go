package database

import "time"

type Board struct {
	Id   int
	Slug string
	Name string
	Tag  string
}

type Thread struct {
	Id        int
	BoardSlug string
	Subject   string
	CreatedAt time.Time
	BumpedAt  time.Time
	Pinned    bool
	Locked    bool
}

type Post struct {
	Id        int
	ThreadId  int
	Author    string
	Body      string
	CreatedAt time.Time
	MediaPath string
	ThumbPath string
	IpHash    string
	Number    int
	Banned    bool
}

type Admin struct {
	Username string
	Password string
}

type Ban struct {
	IpHash     string
	Reason     string
	Expiration time.Time
}
