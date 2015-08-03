package pm

import (
	"container/list"
	"log"
	"sync"
)

/**
Cmd queue manager is used for sequential cmds exectuions
*/
type CmdQueueManager struct {
	queues   map[string]*list.List
	signal   chan string
	consumer chan *Cmd
	producer chan *Cmd
}

/**
Create a new instace of the queue manager. Normally only the PM should call this.
*/
func NewCmdQueueManager() *CmdQueueManager {
	mgr := &CmdQueueManager{
		queues:   make(map[string]*list.List),
		signal:   make(chan string),
		consumer: make(chan *Cmd),
		producer: make(chan *Cmd),
	}

	var lock sync.Mutex
	//start queue dispatcher
	go func() {
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
			lock.Lock()
			queue, ok := mgr.queues[queueName]
			if !ok {
				log.Println("Queue", queueName, "doesn't exist, initializing...")
				queue = list.New()
				mgr.queues[queueName] = queue
			}
			lock.Unlock()
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
	}()

	//start the producer
	go func() {
		for {
			queueName := <-mgr.signal

			log.Println("Queue", queueName, "ready...")
			lock.Lock()
			queue, ok := mgr.queues[queueName]
			if !ok {
				//Proceed on a queue that doesn't exist anymore.
				lock.Unlock()
				continue
			}

			if queue.Len() == 0 {
				//last command on this queue exited successfully.
				//we can safely delete it.
				log.Println("Queue", queueName, "is empty, cleaning up...")
				delete(mgr.queues, queueName)
				lock.Unlock()
				continue
			}

			lock.Unlock()

			next := queue.Remove(queue.Front()).(*Cmd)
			mgr.producer <- next
		}
	}()

	return mgr
}

func (qm *CmdQueueManager) Push(cmd *Cmd) {
	qm.consumer <- cmd
}

func (qm *CmdQueueManager) Notify(cmd *Cmd) {
	queueName := cmd.Args.GetString("queue")
	if queueName == "" {
		//nothing to do
		return
	}

	qm.signal <- queueName
}

func (qm *CmdQueueManager) Producer() <-chan *Cmd {
	return qm.producer
}
