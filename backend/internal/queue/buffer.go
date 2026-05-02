package queue

type DataQueue struct {
    Pipe chan string
}

func New(size int) *DataQueue {
    return &DataQueue{
        Pipe: make(chan string, size),
    }
}
