package util

import cloudflare "github.com/cloudflare/cloudflare-go"

//順番を保持したMap
type OrderedMap struct {
	data map[string]cloudflare.UnvalidatedIngressRule
	keys []string
}

func NewOrderedMap(cap int) *OrderedMap {
	return &OrderedMap{
		data: make(map[string]cloudflare.UnvalidatedIngressRule, cap),
		keys: make([]string, 0, cap),
	}
}

func (n *OrderedMap) Add(s cloudflare.UnvalidatedIngressRule) {
	if _, ok := n.data[s.Hostname]; !ok {
		n.data[s.Hostname] = s
		n.keys = append(n.keys, s.Hostname)
	} else {
		n.data[s.Hostname] = s
		for i, v := range n.keys {
			if v == s.Hostname {
				n.keys = append(n.keys[:i], n.keys[i+1:]...)
				break
			}
		}
		n.keys = append(n.keys, s.Hostname)
	}
}

func (n *OrderedMap) Update(s cloudflare.UnvalidatedIngressRule) {
	if _, ok := n.data[s.Hostname]; ok {
		n.data[s.Hostname] = s
	} else {
		n.data[s.Hostname] = s
		n.keys = append(n.keys, s.Hostname)
	}
}

func (n *OrderedMap) Remove(s cloudflare.UnvalidatedIngressRule) {
	if _, ok := n.data[s.Hostname]; ok {
		delete(n.data, s.Hostname)
		for i, v := range n.keys {
			if v == s.Hostname {
				n.keys = append(n.keys[:i], n.keys[i+1:]...)
				break
			}
		}
	}
}

func (n *OrderedMap) Get() []cloudflare.UnvalidatedIngressRule {
	results := make([]cloudflare.UnvalidatedIngressRule, 0, len(n.keys))
	for _, v := range n.keys {
		results = append(results, n.data[v])
	}
	return results
}

func (n *OrderedMap) Len() int {
	return len(n.keys)
}
