package main

import (
	"bytes"
	"errors"
	"fmt"

	"time"
)

var (
	InvalidCommit = errors.New("File at path cannot be parsed into a commit.")
)

type Commit struct {
	timestamp time.Time
	parent    *string
	tree      string
	author    string
	email     string
	message   *string
}

func (c *Commit) Kind() ObjectKind {
	return CommitKind
}

func (c *Commit) Content() []byte {
	buf := bytes.Buffer{}
	timestamp := fmt.Sprintf("%d %s", c.timestamp.Unix(), c.timestamp.Format("-0700"))
	buf.WriteString(fmt.Sprintf("tree %s\n", c.tree))
	if c.parent != nil {
		buf.WriteString(fmt.Sprintf("parent %s\n", *c.parent))
	}
	buf.WriteString(fmt.Sprintf("author %s <%s> %s\n", c.author, c.email, timestamp))
	buf.WriteString(fmt.Sprintf("committer %s <%s> %s\n\n", c.author, c.email, timestamp))
	if c.message != nil {
		buf.WriteString(*c.message)
	}
	buf.WriteString("\n")

	return buf.Bytes()
}

func (c *Commit) String() string {
	return ""
}
