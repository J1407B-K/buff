package buff

import (
	"fmt"
	"strings"
)

type node struct {
	part     string
	children map[string]*node
	pchild   *node // :param (最多一个)
	schild   *node // *splat (最多一个，且终止)
	wildcard bool
	splat    bool
	handlers map[string]Handler // method -> handler
	tpls     map[string]string  // method -> route template
}

func newNode(part string) *node {
	return &node{part: part, children: map[string]*node{}, handlers: map[string]Handler{}, tpls: map[string]string{}}
}

func (n *node) add(method string, parts []string, h Handler) error {
	if len(parts) == 0 {
		if _, ok := n.handlers[method]; ok {
			return fmt.Errorf("route already exists for %s", method)
		}
		n.handlers[method] = h
		return nil
	}
	p := parts[0]
	switch {
	case strings.HasPrefix(p, ":"):
		if n.pchild == nil {
			n.pchild = newNode(p)
			n.pchild.wildcard = true
		}
		return n.pchild.add(method, parts[1:], h)
	case strings.HasPrefix(p, "*"):
		if len(parts) > 1 {
			return fmt.Errorf("splat must be terminal: %v", parts)
		}
		if n.schild == nil {
			n.schild = newNode(p)
			n.schild.splat = true
		}
		return n.schild.add(method, nil, h)
	default:
		ch := n.children[p]
		if ch == nil {
			ch = newNode(p)
			n.children[p] = ch
		}
		return ch.add(method, parts[1:], h)
	}
}

func (n *node) locate(parts []string) *node {
	if len(parts) == 0 {
		return n
	}
	p := parts[0]
	switch {
	case strings.HasPrefix(p, ":"):
		if n.pchild == nil || n.pchild.part != p {
			return nil
		}
		return n.pchild.locate(parts[1:])
	case strings.HasPrefix(p, "*"):
		if n.schild == nil || n.schild.part != p {
			return nil
		}
		return n.schild
	default:
		ch := n.children[p]
		if ch == nil {
			return nil
		}
		return ch.locate(parts[1:])
	}
}

func (n *node) find(parts []string, params []paramKV) (*node, []paramKV) {
	if len(parts) == 0 {
		return n, params
	}
	p := parts[0]
	// 静态优先
	if ch := n.children[p]; ch != nil {
		if leaf, ps := ch.find(parts[1:], params); leaf != nil {
			return leaf, ps
		}
	}
	// *splat
	if n.schild != nil {
		key := strings.TrimPrefix(n.schild.part, "*")
		ps := append(params, paramKV{key: key, val: strings.Join(parts, "/")})
		return n.schild, ps
	}
	// :param
	if n.pchild != nil {
		key := strings.TrimPrefix(n.pchild.part, ":")
		return n.pchild.find(parts[1:], append(params, paramKV{key: key, val: p}))
	}
	return nil, nil
}
