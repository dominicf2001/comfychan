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
}

type Post struct {
	Id        int
	ThreadId  int
	Author    string
	Body      string
	CreatedAt time.Time
	MediaPath string
	IpHash    string
	Number    int
}
