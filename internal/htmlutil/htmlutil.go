package htmlutil

import (
	"errors"
	"strings"

	"golang.org/x/net/html"
)

const (
	AttributeKeyClass = "class"
)

func Find(n *html.Node, conditions ...FindCondition) (*html.Node, bool) {
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

type FindCondition func(*html.Node) bool

func doesMeetConditions(n *html.Node, conditions ...FindCondition) bool {
	for _, c := range conditions {
		if !c(n) {
			return false
		}
	}
	return true
}

func WithClassEqual(value string) FindCondition {
	return func(n *html.Node) bool {
		return ClassEquals(n, value)
	}
}

func WithClassContaining(values ...string) FindCondition {
	return func(n *html.Node) bool {
		return ClassContains(n, values...)
	}
}

func WithAttributeEqual(key, value string) FindCondition {
	return func(n *html.Node) bool {
		return AttributeEquals(n, key, value)
	}
}

func ClassEquals(n *html.Node, value string) bool {
	return AttributeEquals(n, AttributeKeyClass, value)
}

func ClassContains(n *html.Node, values ...string) bool {
	for _, v := range values {
		if !AttributeContains(n, AttributeKeyClass, v) {
			return false
		}
	}
	return true
}

func AttributeEquals(n *html.Node, key, value string) bool {
	attr, ok := Attribute(n, key)
	if !ok {
		return false
	}
	return attr.Val == value
}

func AttributeContains(n *html.Node, key, value string) bool {
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

func ForEach(n *html.Node, statement ForEachStatement) error {
	if err := forEach(n, statement); err != nil && err != ErrForEachStopped {
		return err
	}
	return nil
}

type ForEachStatement func(*html.Node) error

var ErrForEachStopped = errors.New("for-each loop was stopped")

func forEach(n *html.Node, statement ForEachStatement) error {
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
