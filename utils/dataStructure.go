package utils

type Deque interface {
	Len() int
	// GetFrontElem 获取队首元素
	GetFrontElem() interface{}
	// GetRearElem 获取队尾元素
	GetRearElem() interface{}
	// EnterQueue 元素进队
	EnterQueue(v interface{})
	// RemoveQueue 元素出队
	RemoveQueue() interface{}
}

type deque struct {
	data   interface{}
	next   *deque
	before *deque
}

type DequeImpl struct {
	front *deque
	rear  *deque
	size  int
}

func (d *DequeImpl) RemoveQueue() interface{} {
	if d.size == 0 {
		return nil
	}

	data := d.front.data
	d.front = d.front.next
	if d.front != nil {
		d.front.before = nil
	} else {
		d.rear = nil
	}
	d.size--
	return data
}

func NewDeque() Deque {
	return &DequeImpl{
		front: nil,
		rear:  nil,
		size:  0,
	}
}

func (d *DequeImpl) Len() int {
	return d.size
}

func (d *DequeImpl) GetFrontElem() interface{} {
	if d.front == nil {
		return nil
	}
	return d.front.data
}

func (d *DequeImpl) GetRearElem() interface{} {
	if d.rear == nil {
		return nil
	}
	return d.rear.data
}

func (d *DequeImpl) EnterQueue(v interface{}) {
	newNode := &deque{data: v}

	if d.size == 0 {
		d.front = newNode
		d.rear = newNode
	} else {
		newNode.before = d.rear
		d.rear.next = newNode
		d.rear = newNode
	}
	d.size++
}
