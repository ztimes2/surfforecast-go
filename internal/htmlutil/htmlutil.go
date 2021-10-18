package htmlutil

import (
	"strings"

	"golang.org/x/net/html"
)

const (
	// AttributeClass is the key of the class attribute.
	AttributeClass = "class"

	// AttributeID is the key of the id attribute.
	AttributeID = "id"

	// AttributeAlternateImageText is the key of the alt attribute.
	AttributeAlternateImageText = "alt"

	// AttributeTransform is the key of the transform attribute.
	AttributeTransform = "transform"
)

// Find walks through the given node and all its childen and returns those that
// match the given conditions.
func Find(n *html.Node, conditions ...FindCondition) []*html.Node {
	var targets []*html.Node

	if matchesConditions(n, conditions...) {
		targets = append(targets, n)
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		targets = append(targets, Find(c, conditions...)...)
	}

	return targets
}

// FindOne walks through the given node and all its childen and returns the first
// one that matches the given conditions.
func FindOne(n *html.Node, conditions ...FindCondition) (*html.Node, bool) {
	if matchesConditions(n, conditions...) {
		return n, true
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		target, ok := FindOne(c, conditions...)
		if ok {
			return target, true
		}
	}

	return nil, false
}

// FindCondition is a function that is used for describing a match condition when
// finding nodes.
type FindCondition func(*html.Node) bool

func matchesConditions(n *html.Node, conditions ...FindCondition) bool {
	for _, c := range conditions {
		if !c(n) {
			return false
		}
	}
	return true
}

// WithClassEqual returns FindCondition that checks if a node's class attribute
// equals to the given value.
func WithClassEqual(value string) FindCondition {
	return func(n *html.Node) bool {
		return ClassEquals(n, value)
	}
}

// WithClassContaining returns FindCondition that checks if a node's class attribute
// contains the given values.
func WithClassContaining(values ...string) FindCondition {
	return func(n *html.Node) bool {
		return ClassContains(n, values...)
	}
}

// WithIDEqual returns FindCondition that checks if a node's id attribute equals
// to the given value.
func WithIDEqual(value string) FindCondition {
	return func(n *html.Node) bool {
		return IDEquals(n, value)
	}
}

// WithAttributeEqual returns FindCondition that checks if a node's attribute equals
// to the given value.
func WithAttributeEqual(key, value string) FindCondition {
	return func(n *html.Node) bool {
		return AttributeEquals(n, key, value)
	}
}

// WithAttribute returns FindCondition that checks if a node has the given attribute.
func WithAttribute(key string) FindCondition {
	return func(n *html.Node) bool {
		_, ok := Attribute(n, key)
		return ok
	}
}

// ClassEquals checks if the given node's class attribute equals to the given value.
func ClassEquals(n *html.Node, value string) bool {
	return AttributeEquals(n, AttributeClass, value)
}

// ClassContains checks if the given node's class attribute contains the given value.
func ClassContains(n *html.Node, values ...string) bool {
	for _, v := range values {
		if !AttributeContains(n, AttributeClass, v) {
			return false
		}
	}
	return true
}

// IDEquals checks if the given node's id attribute equals to the given value.
func IDEquals(n *html.Node, value string) bool {
	return AttributeEquals(n, AttributeID, value)
}

// AttributeEquals checks if the given node's attribute equals to the given value.
func AttributeEquals(n *html.Node, key, value string) bool {
	attr, ok := Attribute(n, key)
	if !ok {
		return false
	}
	return attr.Val == value
}

// AttributeContains checks if the given node's attribute contains the given value.
func AttributeContains(n *html.Node, key, value string) bool {
	attr, ok := Attribute(n, key)
	if !ok {
		return false
	}
	return strings.Contains(attr.Val, value)
}

// Attribute looks up for an attribute of the given node.
func Attribute(n *html.Node, key string) (html.Attribute, bool) {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr, true
		}
	}
	return html.Attribute{}, false
}

// ForEach walks through the given node and all of its children, and executes the
// given statement for each of them. The loop runs until all the nodes are visited
// or the statement returns an error.
func ForEach(n *html.Node, statement ForEachStatement) error {
	if err := statement(n); err != nil {
		return err
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if err := ForEach(c, statement); err != nil {
			return err
		}
	}

	return nil
}

// ForEachStatement is a function that is used for describing a statement that gets
// executed for each iteration of ForEach loop. When a non-nil error is returned,
// the early termination of the loop gets triggered.
type ForEachStatement func(*html.Node) error
