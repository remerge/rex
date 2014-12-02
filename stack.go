package rex

type BoolStack []bool

func (s BoolStack) Empty() bool  { return len(s) == 0 }
func (s BoolStack) Peek() bool   { return s[len(s)-1] }
func (s *BoolStack) Push(i bool) { (*s) = append((*s), i) }
func (s *BoolStack) Pop() bool {
	d := (*s)[len(*s)-1]
	(*s) = (*s)[:len(*s)-1]
	return d
}

type IntStack []int

func (s IntStack) Empty() bool { return len(s) == 0 }
func (s IntStack) Peek() int   { return s[len(s)-1] }
func (s *IntStack) Push(i int) { (*s) = append((*s), i) }
func (s *IntStack) Pop() int {
	d := (*s)[len(*s)-1]
	(*s) = (*s)[:len(*s)-1]
	return d
}
