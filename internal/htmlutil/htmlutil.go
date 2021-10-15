package htmlutil

import (
	"errors"
	"strings"

	"golang.org/x/net/html"
)

func Find(n *html.Node, conditions ...Condition) (*html.Node, bool) {
	if doesMeetConditions(n, conditions...) {
		return n, true
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		target, ok := Find(c, conditions...)
		if ok {
			return target, true
		}
	}
	return nil, false
}

type Condition func(*html.Node) bool

func doesMeetConditions(n *html.Node, conditions ...Condition) bool {
	for _, c := range conditions {
		if !c(n) {
			return false
		}
	}
	return true
}

func WithAttributeValue(key, value string) Condition {
	return func(n *html.Node) bool {
		return AttributeEqualsValue(n, key, value)
	}
}

func AttributeEqualsValue(n *html.Node, key, value string) bool {
	attr, ok := Attribute(n, key)
	if !ok {
		return false
	}
	return attr.Val == value
}

func AttributeContainsValue(n *html.Node, key, value string) bool {
	attr, ok := Attribute(n, key)
	if !ok {
		return false
	}
	return strings.Contains(attr.Val, value)
}

func Attribute(n *html.Node, key string) (html.Attribute, bool) {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr, true
		}
	}
	return html.Attribute{}, false
}

func ForEach(n *html.Node, statement Statement) error {
	if err := forEach(n, statement); err != nil && err != ErrLoopStopped {
		return err
	}
	return nil
}

type Statement func(*html.Node) error

var ErrLoopStopped = errors.New("loop was stopped")

func forEach(n *html.Node, statement Statement) error {
	if err := statement(n); err != nil {
		return err
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := forEach(c, statement); err != nil {
			return err
		}
	}

	return nil
}
