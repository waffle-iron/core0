package pm

import (
	"container/list"
	"github.com/g8os/core/agent/lib/pm/core"
	"log"
	"sync"
)

/**
cmdQueueManager is used for sequential cmds exectuions
*/
type cmdQueueManager struct {
	queues   map[string]*list.List
	signal   chan string
	consumer chan *core.Cmd
	producer chan *core.Cmd
	lock     sync.Mutex
}

/*
NewCmdQueueManager creates a new instace of the queue manager. Normally only the PM should call this.
*/
func newCmdQueueManager() *cmdQueueManager {
	mgr := &cmdQueueManager{
		queues:   make(map[string]*list.List),
		signal:   make(chan string),
		consumer: make(chan *core.Cmd),
		producer: make(chan *core.Cmd),
	}

	//start queue dispatcher
	go mgr.dispatcherLoop()

	//start the producer
	go mgr.producerLoop()

	return mgr
}

func (mgr *cmdQueueManager) dispatcherLoop() {
	for {
		cmd := <-mgr.consumer

		queueName := cmd.Args.GetString("queue")
		if queueName == "" {
			//not a queued command just log a warning and continue
			log.Println("WARNING: Queue manager received a command with no set queue", cmd)
			continue
		}

		log.Println("Pushing command to queue", queueName)
		//push command to correct queue
		mgr.lock.Lock()
		queue, ok := mgr.queues[queueName]
		if !ok {
			log.Println("Queue", queueName, "doesn't exist, initializing...")
			queue = list.New()
			mgr.queues[queueName] = queue
		}
		mgr.lock.Unlock()
		//push the command to the queue.
		queue.PushBack(cmd)

		if !ok {
			//since we just create this queue. We signal that it's ready
			//to produce the first queued command. Think of it as intial start
			//condition. When this command exists, it will auto signal the next command
			//and so on.
			mgr.signal <- queueName
		}
	}
}

//producer gets the next job to run and send it over channel
func (mgr *cmdQueueManager) producerLoop() {
	for {
		queueName := <-mgr.signal

		log.Println("Queue", queueName, "ready...")
		mgr.lock.Lock()
		queue, ok := mgr.queues[queueName]
		if !ok {
			//Proceed on a queue that doesn't exist anymore.
			mgr.lock.Unlock()
			continue
		}

		if queue.Len() == 0 {
			//last command on this queue exited successfully.
			//we can safely delete it.
			log.Println("Queue", queueName, "is empty, cleaning up...")
			delete(mgr.queues, queueName)
			mgr.lock.Unlock()
			continue
		}

		mgr.lock.Unlock()

		next := queue.Remove(queue.Front()).(*core.Cmd)
		mgr.producer <- next
	}
}

func (mgr *cmdQueueManager) Push(cmd *core.Cmd) {
	mgr.consumer <- cmd
}

func (mgr *cmdQueueManager) Notify(cmd *core.Cmd) {
	queueName := cmd.Args.GetString("queue")
	if queueName == "" {
		//nothing to do
		return
	}

	mgr.signal <- queueName
}

func (mgr *cmdQueueManager) Producer() <-chan *core.Cmd {
	return mgr.producer
}
