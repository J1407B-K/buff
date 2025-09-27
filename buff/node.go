package buff

import (
	"fmt"
	"strings"
)

type node struct {
	part     string
	pchild   *node // :param (最多一个)
	schild   *node // *splat (最多一个，且终止)
	children map[string]*node
	wildcard bool
	splat    bool
	handlers map[string]Handler // method -> handler
	tpls     map[string]string  // method -> route template
}

func newNode(part string) *node {
	return &node{part: part, children: map[string]*node{}, handlers: map[string]Handler{}, tpls: map[string]string{}}
}

func (n *node) add(method string, parts []string, h Handler, tpl string) error {
	if len(parts) == 0 {
		if _, ok := n.handlers[method]; ok {
			return fmt.Errorf("route already exists for %s", method)
		}
		n.handlers[method] = h
		n.tpls[method] = tpl
		return nil
	}
	p := parts[0]
	switch {
	case strings.HasPrefix(p, ":"):
		if n.pchild == nil {
			n.pchild = newNode(p)
			n.pchild.wildcard = true
		}
		return n.pchild.add(method, parts[1:], h, tpl)
	case strings.HasPrefix(p, "*"):
		if len(parts) > 1 {
			return fmt.Errorf("splat must be terminal: %v", parts)
		}
		if n.schild == nil {
			n.schild = newNode(p)
			n.schild.splat = true
		}
		return n.schild.add(method, nil, h, tpl)
	default:
		ch := n.children[p]
		if ch == nil {
			ch = newNode(p)
			n.children[p] = ch
		}
		return ch.add(method, parts[1:], h, tpl)
	}
}

func (n *node) findPath(path string, i, j int, params []paramKV) (*node, []paramKV) {
	if i >= j {
		return n, params
	}
	k := i
	for k < j && path[k] != '/' {
		k++
	}
	seg := path[i:k]

	// static
	if ch := n.children[seg]; ch != nil {
		return ch.findPath(path, nextIndex(k, j), j, params)
	}

	// splat
	if n.schild != nil {
		key := strings.TrimPrefix(n.schild.part, "*")
		params = append(params, paramKV{key: key, val: path[i:j]})
		return n.schild, params
	}
	// param
	if n.pchild != nil {
		key := strings.TrimPrefix(n.pchild.part, ":")
		params = append(params, paramKV{key: key, val: seg})
		return n.pchild.findPath(path, nextIndex(k, j), j, params)
	}
	return nil, params
}

func nextIndex(k, j int) int {
	if k < j && j > 0 && k+1 <= j {
		return k + 1
	}
	return k
}
