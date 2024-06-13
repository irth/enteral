package main

import "encoding/xml"

type Feed struct {
	XMLName    xml.Name `xml:"feed"`
	Xmlns      string   `xml:"xmlns,attr"`
	XmlnsMedia string   `xml:"xmlns:media,attr"`
	XmlnsYt    string   `xml:"xmlns:yt,attr"`
	Id         string   `xml:"id"`
	Title      string   `xml:"title"`
	Links      []Link
	Author     *Author `xml:"author,omitempty"`
	Published  string  `xml:"published,omitempty"`
	Entries    []Entry `xml:"entry"`
}

type Link struct {
	XMLName xml.Name `xml:"link"`
	Href    string   `xml:"href,attr"`
	Rel     string   `xml:"rel,attr,omitempty"`
}

type Author struct {
	XMLName xml.Name `xml:"author"`
	Name    string   `xml:"name"`
	Uri     string   `xml:"uri"`
}

type Entry struct {
	ID        string  `xml:"id"`
	Title     string  `xml:"title"`
	Author    *Author `xml:"author,omitempty"`
	Published string  `xml:"published,omitempty"`
	Updated   string  `xml:"updated,omitempty"`
	Link      Link
}
