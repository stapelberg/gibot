// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package site

type Repo struct {
	Name    string
	Path    string
	Aliases []string
	Token   string
}
