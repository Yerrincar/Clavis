package ds

type Node struct {
	Key  string
	prev *Node
	next *Node
}

type LRUCache struct {
	Cache map[string]*Node
	head  *Node
	Tail  *Node
}

func NewLRUCache() *LRUCache {
	return &LRUCache{
		Cache: make(map[string]*Node),
	}
}

func (l *LRUCache) MoveToFront(node *Node) {

	if node == nil {
		return
	}

	if l.head == node {
		return
	}

	if node.prev != nil {
		node.prev.next = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	}

	if l.Tail == node {
		l.Tail = node.prev
	}

	node.next = l.head
	node.prev = nil

	if l.head != nil {
		l.head.prev = node
	}

	l.head = node
	if l.Tail == nil {
		l.Tail = node
	}
}

func (l *LRUCache) Evict(excludedKey string) (string, bool) {
	victim := l.Tail

	if victim != nil && victim.Key == excludedKey {
		victim = victim.prev
	}

	if victim == nil {
		return "", false
	}
	if victim.prev != nil {
		victim.prev.next = victim.next
	} else {
		l.head = victim.next
	}

	if victim.next != nil {
		victim.next.prev = victim.prev
	} else {
		l.Tail = victim.prev
	}

	delete(l.Cache, victim.Key)

	victim.prev = nil
	victim.next = nil

	return victim.Key, true
}

func (l *LRUCache) Put(key string) {
	if _, exists := l.Cache[key]; exists {
		l.MoveToFront(l.Cache[key])
		return
	}

	newNode := &Node{Key: key}
	newNode.next = l.head

	if l.head != nil {
		l.head.prev = newNode
	}

	l.head = newNode
	if l.Tail == nil {
		l.Tail = newNode
	}

	l.Cache[key] = newNode
}

func (l *LRUCache) Remove(key string) bool {
	victim := l.Cache[key]

	if victim == nil {
		return false
	}
	if victim.prev != nil {
		victim.prev.next = victim.next
	} else {
		l.head = victim.next
	}

	if victim.next != nil {
		victim.next.prev = victim.prev
	} else {
		l.Tail = victim.prev
	}

	delete(l.Cache, victim.Key)

	victim.prev = nil
	victim.next = nil

	return true

}
