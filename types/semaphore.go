package types

type empty struct{}
type Semaphore chan empty

//Not blocking!
func (s Semaphore) P(n int) {
	e := empty{}
	for i := 0; i < n; i++ {
		select {
		case s <- e:
		default:
			//semaphore is full
		}
	}
}

func (s Semaphore) V(n int) {
	for i := 0; i < n; i++ {
		<-s
	}
}
